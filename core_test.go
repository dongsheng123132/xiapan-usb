package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestResourceHashRequiredBeforeExecution(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "models"), 0700)
	os.WriteFile(filepath.Join(root, "models", "test.gguf"), []byte("changed"), 0600)
	hash := sha256.Sum256([]byte("expected"))
	if _, err := verifyResource(root, "models/test.gguf", hex.EncodeToString(hash[:])); err == nil {
		t.Fatal("accepted altered model resource")
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
