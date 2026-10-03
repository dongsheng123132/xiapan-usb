package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPortableToolVerification(t *testing.T) {
	root := t.TempDir()
	body := []byte("fixture executable, never run")
	h := sha256.Sum256(body)
	tool := PortableTool{ID: "fixture", Name: "fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Directory: "app/tools/fixture", Executable: "tool.exe", Files: map[string]string{"tool.exe": hex.EncodeToString(h[:])}}
	dir := filepath.Join(root, "app", "tools", "fixture")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "tool.exe"), body, 0600)
	if _, err := verifyPortableTool(context.Background(), root, tool); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "tool.exe"), []byte("modified"), 0600)
	if _, err := verifyPortableTool(context.Background(), root, tool); err == nil {
		t.Fatal("tampered executable accepted")
	}
	tool.Executable = "../outside.exe"
	if _, err := verifyPortableTool(context.Background(), root, tool); err == nil {
		t.Fatal("unlisted entrypoint accepted")
	}
	if _, err := portableToolPath(root, tool, "../outside.exe"); err == nil {
		t.Fatal("path traversal accepted")
	}
	tool.Platform = "unsupported/platform"
	if _, err := verifyPortableTool(context.Background(), root, tool); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestToolLibraryDoesNotClaimUnpreparedTools(t *testing.T) {
	value, err := toolCatalog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog := value.(map[string]any)
	for _, row := range catalog["portable_tools"].([]any) {
		if row.(map[string]any)["ready"] != false {
			t.Fatal("missing portable tool reported ready")
		}
	}
	// Decode the exact public keys, including official_url, without relying on UI text.
	b, _ := assets.ReadFile("catalog/tool-library.json")
	var raw []map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tool := range portableTools() {
		seen[tool.ID] = true
	}
	for _, row := range raw {
		id, _ := row["id"].(string)
		if id == "" || seen[id] || row["ready"] != false {
			t.Fatal("invalid or duplicate catalog row", id)
		}
		seen[id] = true
		for _, key := range []string{"name", "category", "description", "portable_note", "reason"} {
			if text, ok := row[key].(string); !ok || text == "" {
				t.Fatal("missing tool metadata", id, key)
			}
		}
		if url, ok := row["official_url"].(string); !ok || !strings.HasPrefix(url, "https://") {
			t.Fatal("invalid official link", id)
		}
		if _, err := launchRegisteredTool(context.Background(), t.TempDir(), id, true); err == nil {
			t.Fatal("download-only tool executable", id)
		}
	}
}

func TestPortableToolsRequireExplicitConfirmationAndPackage(t *testing.T) {
	root := t.TempDir()
	tools := portableTools()
	if len(tools) != 6 {
		t.Fatal("expected six pinned portable tools")
	}
	for _, tool := range tools {
		if _, err := launchRegisteredTool(context.Background(), root, tool.ID, false); err == nil {
			t.Fatal("unconfirmed launch accepted")
		}
		if _, err := launchRegisteredTool(context.Background(), root, tool.ID, true); err == nil {
			t.Fatal("missing package launch accepted")
		}
	}
}
