package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableModelSelectionAndCredentialIsolation(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	get, err := modelSettingsAction(ctx, root, "settings.model.get", nil)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"expected_revision": get.(map[string]any)["revision"], "source": "custom", "model": "org/my-model", "base_url": "https://example.org/v1/", "api_key": "sk-personal-fixture"}
	saved, err := modelSettingsAction(ctx, root, "settings.model.save", input)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(saved)
	if strings.Contains(string(b), "sk-personal") {
		t.Fatal("settings response leaked credential")
	}
	if _, err = modelSettingsAction(ctx, root, "settings.model.save", input); err == nil {
		t.Fatal("stale settings overwrite accepted")
	}
	s, err := selectedModel(root)
	if err != nil {
		t.Fatal(err)
	}
	config := picoConfigForModel(filepath.Join(root, "data", "pico", "workspace"), s)
	m := config["model_list"].([]any)[0].(map[string]any)
	if m["model"] != "org/my-model" || m["provider"] != "openai" || m["api_base"] != "https://example.org/v1" {
		t.Fatalf("selection lost: %#v", m)
	}
	input["expected_revision"] = s.Revision
	input["api_key"] = ""
	input["model"] = "org/another-model"
	if _, err = modelSettingsAction(ctx, root, "settings.model.save", input); err != nil {
		t.Fatal(err)
	}
	s, _ = selectedModel(root)
	if s.APIKey != "sk-personal-fixture" {
		t.Fatal("blank edit lost existing key")
	}
	input["expected_revision"] = s.Revision
	input["base_url"] = "https://different.example/v1"
	if _, err = modelSettingsAction(ctx, root, "settings.model.save", input); err != nil {
		t.Fatal(err)
	}
	s, _ = selectedModel(root)
	if s.APIKey != "" {
		t.Fatal("old key carried to a new endpoint")
	}
	backups, _ := filepath.Glob(filepath.Join(root, "data", "backups", "*model.json"))
	if len(backups) == 0 {
		t.Fatal("settings changed without recovery backup")
	}
	// A directory relocation must retain the user's choice, not an absolute home.
	dest := filepath.Join(t.TempDir(), "换盘后的精灵")
	if err = os.Rename(root, dest); err != nil {
		t.Fatal(err)
	}
	s, err = selectedModel(dest)
	if err != nil || s.Model != "org/another-model" {
		t.Fatalf("portable settings lost: %v %#v", err, s)
	}
}

func TestModelConnectionTestDoesNotSaveAndUsesOnlySelectedKey(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-test-custom" {
			t.Errorf("wrong endpoint or key: %s", r.URL.Path)
		}
		var request map[string]any
		json.NewDecoder(r.Body).Decode(&request)
		if request["model"] != "fixture-model" {
			t.Error("wrong model")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"connection_check"}}]}}]}`))
	}))
	defer server.Close()
	if err := saveWallet(root, WalletState{APIKey: "sk-wallet-must-not-be-sent"}); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"expected_revision": "initial", "source": "custom", "model": "fixture-model", "base_url": server.URL + "/v1", "api_key": "sk-test-custom"}
	if _, err := modelSettingsAction(ctx, root, "settings.model.test", input); err != nil {
		t.Fatal(err)
	}
	s, _ := selectedModel(root)
	if calls != 1 || s.Revision != "initial" || s.Source != "cloud" {
		t.Fatal("test changed saved selection")
	}
	input["base_url"] = "http://remote.example/v1"
	if _, err := modelSettingsAction(ctx, root, "settings.model.save", input); err == nil {
		t.Fatal("insecure remote endpoint accepted")
	}
	if _, err := modelSettingsAction(ctx, root, "settings.model.test", input); err == nil || calls != 1 {
		t.Fatal("invalid endpoint contacted")
	}
}

func TestModelSettingsCorruptionAndModelToolBoundary(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "data", "settings", "model.json")
	os.MkdirAll(filepath.Dir(p), 0700)
	os.WriteFile(p, []byte("broken"), 0600)
	if _, err := selectedModel(root); err == nil {
		t.Fatal("corrupt settings ignored")
	}
	if _, err := modelSettingsAction(context.Background(), root, "settings.model.save", map[string]any{"expected_revision": "initial"}); err == nil {
		t.Fatal("corrupt settings overwritten")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "broken" {
		t.Fatal("corrupt original lost")
	}
	s := ChatSession{ID: uniqueID()}
	result := modelTool(context.Background(), root, &s, "settings.model.save", map[string]any{"source": "custom"})
	b, _ = json.Marshal(result)
	if !strings.Contains(string(b), `"ok":false`) {
		t.Fatal("model can change its own provider")
	}
	if !chatReadActions["startup.inspect"] {
		t.Fatal("startup evidence unavailable to agent")
	}
}

func TestCompactPackageRuntimeInventory(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "app", "runtime", "picoclaw", "windows-x64", "picoclaw.exe")
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("inventory fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	scan, err := scanResources(root)
	if err != nil || len(scan.Warnings) != 0 || len(scan.Files) != 1 || scan.Files[0].Path != "app/runtime/picoclaw/windows-x64/picoclaw.exe" {
		t.Fatalf("compact runtime not discoverable: %#v %v", scan, err)
	}
	resolved, err := runtimeResourcePath(root, "runtime/picoclaw/windows-x64/picoclaw.exe")
	if err != nil || resolved != p {
		t.Fatalf("runtime location drift: %s %v", resolved, err)
	}
}
