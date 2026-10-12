package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func markOfflineTestRoot(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "OFFLINE_ONLY"), nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerOfflineDefaultRequiresBundledResources(t *testing.T) {
	root := t.TempDir()
	if defaultChatEngine(root) != "pico" {
		t.Fatal("ordinary package default changed")
	}
	markOfflineTestRoot(t, root)
	if defaultChatEngine(root) != "guide" {
		t.Fatal("missing model should leave local tools usable, never select cloud")
	}
	m, err := aiManifest()
	if err != nil || m.Platform != runtime.GOOS+"/"+runtime.GOARCH || !m.Ready {
		return
	}
	for _, rel := range []string{m.ModelPath, m.EnginePath} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("inventory only; execution still requires hashes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if defaultChatEngine(root) != "local" {
		t.Fatal("complete bundled inventory did not default to local")
	}
	if _, err := verifyResource(root, m.ModelPath, m.ModelSHA); err == nil {
		t.Fatal("inventory readiness must not bypass runtime hash verification")
	}
}

func TestOfflineEditionRejectsOnlineChatAndUpgradesOldSession(t *testing.T) {
	root := t.TempDir()
	v, err := chatAction(context.Background(), root, "chat.start", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	old := v.(ChatSession)
	markOfflineTestRoot(t, root)
	v, err = chatAction(context.Background(), root, "chat.read", map[string]any{"session_id": old.ID})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(ChatSession)
	if !s.OfflineOnly || s.Engine != "guide" {
		t.Fatalf("old online session was not made offline: %#v", s)
	}
	for _, engine := range []string{"pico", "cloud", "unknown"} {
		_, err = chatAction(context.Background(), root, "chat.send", map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "engine": engine, "message": "电脑没网了"})
		if err == nil || !strings.Contains(err.Error(), "离线模式") {
			t.Fatalf("%s was not blocked before IO: %v", engine, err)
		}
	}
	after, err := loadChat(root, s.ID)
	if err != nil || after.Version != s.Version || len(after.Messages) != 0 {
		t.Fatalf("rejected requests mutated the session: %#v %v", after, err)
	}
	v, err = chatAction(context.Background(), root, "chat.send", map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "message": "我应该先看哪里？"})
	if err != nil {
		t.Fatal(err)
	}
	if next := v.(ChatSession); next.Engine != "guide" || !next.OfflineOnly || len(next.Messages) != 2 {
		t.Fatalf("default request did not remain a usable local guide: %#v", next)
	}
}

func TestOfflineReplyActuallyReceivesKnowledgeEvidenceAndHistory(t *testing.T) {
	root := t.TempDir()
	markOfflineTestRoot(t, root)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		var payload struct {
			Messages []map[string]string `json:"messages"`
			Tools    json.RawMessage     `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		joined, _ := json.Marshal(payload.Messages)
		for _, wanted := range []string{"SMART", "DNS", "SFC", "DISM", "未经维护专项训练", "飞行模式只适用于电脑无线连接", "不要求与 Windows 版本一致", "只有浏览器打不开", "代理程序已经退出", "fixture-adapter"} {
			if !strings.Contains(string(joined), wanted) {
				t.Errorf("missing contextual knowledge/evidence/history: %s", wanted)
			}
		}
		if payload.Tools != nil {
			t.Error("small-model explanation must not receive executable tools")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"先核对原代理设置，再确认代理程序是否运行。"}}]}`))
	}))
	defer server.Close()
	old, existed := modelAddresses.Load("bundled")
	modelAddresses.Store("bundled", server.URL)
	t.Cleanup(func() {
		if existed {
			modelAddresses.Store("bundled", old)
		} else {
			modelAddresses.Delete("bundled")
		}
	})
	question := "代理程序已经退出，接下来怎么判断？"
	s := &ChatSession{Messages: []ChatMessage{
		{Role: "user", Text: "只有浏览器打不开"},
		{Role: "assistant", Text: "先检查代理程序是否运行。"},
		{Role: "user", Text: question},
		{Role: "action", Action: "network.inspect", Data: map[string]any{"adapter": "fixture-adapter"}},
	}}
	reply, err := offlineMaintenanceReply(context.Background(), root, s, question, LocalService{Kind: "bundled", Model: "fixture"}, "zh-CN")
	if err != nil || !called || !strings.Contains(reply, "代理设置") {
		t.Fatalf("local response failed: %q %v", reply, err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "app" {
		t.Fatal("model response unexpectedly changed local files")
	}
}

func TestOfflineReplyRejectsRemoteAndUnrelatedLocalService(t *testing.T) {
	root := t.TempDir()
	markOfflineTestRoot(t, root)
	old, existed := modelAddresses.Load("bundled")
	t.Cleanup(func() {
		if existed {
			modelAddresses.Store("bundled", old)
		} else {
			modelAddresses.Delete("bundled")
		}
	})
	modelAddresses.Store("bundled", "https://example.invalid")
	_, err := offlineMaintenanceReply(context.Background(), root, &ChatSession{}, "测试", LocalService{Kind: "bundled", Model: "fixture"}, "zh-CN")
	if err == nil {
		t.Fatal("remote endpoint was accepted")
	}
	_, err = offlineMaintenanceReply(context.Background(), root, &ChatSession{}, "测试", LocalService{Kind: "llamacpp", Model: "fixture"}, "zh-CN")
	if err == nil || !strings.Contains(err.Error(), "随盘模型") {
		t.Fatalf("unrelated local model was accepted: %v", err)
	}
}
