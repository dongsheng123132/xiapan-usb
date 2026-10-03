package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
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

func TestPortableToolsRequireExplicitConfirmationAndPackage(t *testing.T) {
	root := t.TempDir()
	tools := portableTools()
	if len(tools) != 2 {
		t.Fatal("expected two pinned portable tools")
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
