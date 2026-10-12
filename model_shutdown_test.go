package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestQuitCancelsActiveChatAndWaitsForOwnedCleanup(t *testing.T) {
	root := t.TempDir()
	value, err := chatAction(context.Background(), root, "chat.start", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	session := value.(ChatSession)
	server, err := newServer(root, "http://127.0.0.1:8123")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:8123"
		r.Header.Set("Origin", "http://127.0.0.1:8123")
		r.Header.Set("X-Xiapan-Token", token)
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	index := request("GET", "/", "", "")
	match := regexp.MustCompile(`name="xiapan-token" content="([a-f0-9]+)"`).FindStringSubmatch(index.Body.String())
	if len(match) != 2 {
		t.Fatal("missing session token")
	}
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	old := nativeAgent.Chat
	nativeAgent.Chat = func(ctx context.Context, _ string, _ *ChatSession, _, _ string) (string, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release // Simulate finishing an owned write after cancellation.
		return "", ctx.Err()
	}
	t.Cleanup(func() { nativeAgent.Chat = old })
	body, _ := json.Marshal(map[string]any{"action": "chat.send", "input": map[string]any{"session_id": session.ID, "expected_version": session.Version, "engine": "pico", "message": "测试退出"}})
	chatDone := make(chan struct{})
	go func() { request("POST", "/api/run", match[1], string(body)); close(chatDone) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("chat did not start")
	}
	quitDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { quitDone <- request("POST", "/api/quit", match[1], "") }()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("quit did not cancel active work")
	}
	select {
	case <-quitDone:
		close(release)
		t.Fatal("quit returned success before the cancelled action finished")
	default:
	}
	close(release)
	select {
	case <-chatDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled chat did not finish")
	}
	select {
	case reply := <-quitDone:
		if reply.Code != http.StatusOK {
			t.Fatalf("quit failed: %s", reply.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("quit failed to finish after cleanup")
	}
	if r := request("POST", "/api/run", match[1], `{"action":"chat.start"}`); r.Code != http.StatusServiceUnavailable {
		t.Fatal("new action accepted after shutdown began")
	}
}

func TestModelShutdownHelper(t *testing.T) {
	if os.Getenv("XIAPAN_TEST_MODEL_PROCESS") != "1" {
		return
	}
	time.Sleep(20 * time.Second)
}

func TestStopAllModelsWaitsForOwnedProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestModelShutdownHelper$")
	cmd.Env = append(os.Environ(), "XIAPAN_TEST_MODEL_PROCESS=1")
	quietProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	p := &modelProcess{cmd: cmd, done: make(chan struct{})}
	go func() { cmd.Wait(); close(p.done) }()
	root := t.TempDir()
	modelMu.Lock()
	modelProcesses[root] = p
	modelMu.Unlock()
	if err := stopAllModels(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.done:
	default:
		t.Fatal("stop returned before child Wait completed")
	}
	modelMu.Lock()
	_, retained := modelProcesses[root]
	modelMu.Unlock()
	if retained || cmd.ProcessState == nil {
		t.Fatal("owned process was not reaped and removed")
	}
}
