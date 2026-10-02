package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestResourceScanContainsOnlyResourceRoots(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "models"), 0700)
	os.MkdirAll(filepath.Join(root, "data", "private"), 0700)
	os.WriteFile(filepath.Join(root, "models", "中文模型.gguf"), []byte("fixture"), 0600)
	os.WriteFile(filepath.Join(root, "data", "private", "secret.gguf"), []byte("private"), 0600)
	r, err := scanResources(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 1 || r.Files[0].Path != "models/中文模型.gguf" {
		t.Fatalf("unexpected scan %#v", r)
	}
}

func TestModelProbeRequiresActualModelList(t *testing.T) {
	state := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if state == 0 {
			w.WriteHeader(503)
			w.Write([]byte(`{"error":"loading"}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"fixture-model"}]}`))
	}))
	defer server.Close()
	old := localBackends["llamacpp"]
	localBackends["llamacpp"] = server.URL
	defer func() { localBackends["llamacpp"] = old }()
	if _, err := queryLocalModels(context.Background(), "llamacpp"); err == nil {
		t.Fatal("loading service reported ready")
	}
	state = 1
	rows, err := queryLocalModels(context.Background(), "llamacpp")
	if err != nil || len(rows) != 1 || rows[0].Model != "fixture-model" {
		t.Fatalf("ready list not detected: %v %v", rows, err)
	}
}

func TestResourceHashRequiredBeforeExecution(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "models"), 0700)
	os.WriteFile(filepath.Join(root, "models", "test.gguf"), []byte("changed"), 0600)
	hash := sha256.Sum256([]byte("expected"))
	if _, err := verifyResource(root, "models/test.gguf", hex.EncodeToString(hash[:])); err == nil {
		t.Fatal("accepted altered model resource")
	}
}

func TestModelReplyUsesSkillsWithoutExecutingItsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"data":[{"id":"fixture-model"}]}`))
			return
		}
		var request map[string]any
		json.NewDecoder(r.Body).Decode(&request)
		messages := request["messages"].([]any)
		system := messages[0].(map[string]any)["content"].(string)
		if !strings.Contains(system, "电脑体检与维护分诊") {
			t.Error("maintenance skill missing from model context")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"这里是建议，不执行任何命令。"}}],"usage":{"completion_tokens":12}}`))
	}))
	defer server.Close()
	old := localBackends["llamacpp"]
	localBackends["llamacpp"] = server.URL
	defer func() { localBackends["llamacpp"] = old }()
	root := t.TempDir()
	p, err := maintenancePlan(context.Background(), root, map[string]any{"message": "电脑变慢", "backend": "llamacpp", "model": "fixture-model"})
	if err != nil || p.Mode != "local-model" {
		t.Fatalf("local reply failed: %v", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("model reply changed filesystem")
	}
}

func TestResourceScanSkipsLinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "private.gguf"), []byte("private"), 0600)
	if err := os.Symlink(outside, filepath.Join(root, "models")); err != nil {
		t.Skipf("symlink permission unavailable: %v", err)
	}
	r, err := scanResources(root)
	if err != nil || len(r.Files) != 0 {
		t.Fatalf("link escaped scan: %#v %v", r, err)
	}
}

func TestAtomicNewFileDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	p, err := newFileAtomic(root, []string{"data", "reports", "same.json"}, []byte("original"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = newFileAtomic(root, []string{"data", "reports", "same.json"}, []byte("replacement")); err == nil {
		t.Fatal("overwrote existing file")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "original" {
		t.Fatalf("changed existing state: %s", b)
	}
}

func TestWritePathRejectsTraversalAndLinks(t *testing.T) {
	root := t.TempDir()
	for _, p := range [][]string{{"data", "..", "escape"}, {"data", "/absolute"}, {"data", "C:escape"}, {"data", "a\\b"}} {
		if _, err := writablePath(root, p...); err == nil {
			t.Fatalf("accepted path %v", p)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "data")); err != nil {
		t.Skipf("symlink permission unavailable: %v", err)
	}
	if _, err := newFileAtomic(root, []string{"data", "reports", "bad.json"}, []byte("bad")); err == nil {
		t.Fatal("wrote through link")
	}
}

func TestReportCanBeReadAfterRelocation(t *testing.T) {
	root := t.TempDir()
	result, err := createReport(root)
	if err != nil {
		t.Fatal(err)
	}
	p := result.(map[string]any)["path"].(string)
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	os.MkdirAll(filepath.Dir(filepath.Join(other, filepath.FromSlash(p))), 0700)
	os.WriteFile(filepath.Join(other, filepath.FromSlash(p)), b, 0600)
	rows, err := listReports(other)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.([]any)) != 1 {
		t.Fatalf("lost moved report: %v", rows)
	}
}

func TestInstallRequiresConfirmationAndKnownPackage(t *testing.T) {
	root := t.TempDir()
	r := runAction(context.Background(), root, "tools.install", map[string]any{"package_id": "portable-check"})
	if r.OK {
		t.Fatal("install without confirmation")
	}
	r = runAction(context.Background(), root, "tools.install", map[string]any{"package_id": "../../arbitrary.exe", "confirmed": true})
	if r.OK {
		t.Fatal("installed arbitrary executable")
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatal("failed operation created data")
	}
}

func TestDestructiveRequestOnlyProducesPlan(t *testing.T) {
	root := t.TempDir()
	r := runAction(context.Background(), root, "maintenance.plan", map[string]any{"message": "格式化 F 盘，再关闭防火墙"})
	if !r.OK {
		t.Fatal(r.Error)
	}
	p := r.Data.(Plan)
	if p.Mode != "guide" || p.Action != "resources.scan" {
		t.Fatalf("unexpected action %v", p)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("planning modified disk")
	}
}

func TestLocalModelCannotSelectRemoteEndpoint(t *testing.T) {
	r := runAction(context.Background(), t.TempDir(), "maintenance.plan", map[string]any{"message": "检查电脑", "backend": "http://example.com", "model": "test"})
	if r.OK {
		t.Fatal("accepted arbitrary remote endpoint")
	}
}

func TestHTTPRequiresLocalHostOriginAndToken(t *testing.T) {
	server, err := newServer(t.TempDir(), "http://127.0.0.1:8123")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, host, origin, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if token != "" {
			r.Header.Set("X-Xiapan-Token", token)
		}
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)
		return w
	}
	if r := request("GET", "/", "evil.example", "", "", ""); r.Code != 403 {
		t.Fatal("accepted foreign Host")
	}
	if r := request("GET", "/", "127.0.0.1:8123", "https://evil.example", "", ""); r.Code != 403 {
		t.Fatal("accepted foreign Origin")
	}
	if r := request("POST", "/api/run", "127.0.0.1:8123", "", "", `{"action":"report.create"}`); r.Code != 403 {
		t.Fatal("accepted unauthenticated mutation")
	}
	index := request("GET", "/", "127.0.0.1:8123", "", "", "")
	match := regexp.MustCompile(`name="xiapan-token" content="([a-f0-9]+)"`).FindStringSubmatch(index.Body.String())
	if len(match) != 2 {
		t.Fatal("missing local session token")
	}
	r := request("POST", "/api/run", "127.0.0.1:8123", "http://127.0.0.1:8123", match[1], `{"action":"system.inspect"}`)
	var result Result
	if json.Unmarshal(r.Body.Bytes(), &result) != nil || !result.OK {
		t.Fatalf("legitimate action failed: %s", r.Body.String())
	}
}
