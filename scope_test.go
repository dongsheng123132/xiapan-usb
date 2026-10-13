package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineGuideWorksWithoutWalletOrModel(t *testing.T) {
	root := t.TempDir()
	result := runAction(context.Background(), root, "maintenance.plan", map[string]any{"message": "断网了怎么办"})
	if !result.OK {
		t.Fatal(result.Error)
	}
	plan := result.Data.(Plan)
	if plan.Mode != "guide" || plan.Action != "drivers.inspect" {
		t.Fatalf("offline flow did not reach local diagnostics: %#v", plan)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("offline advice created credentials or model state")
	}
}

func TestResourceScanSkipsFinderMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "drivers")
	os.MkdirAll(dir, 0700)
	for _, name := range []string{"._driver.inf", ".DS_Store", "driver.inf"} {
		os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600)
	}
	scan, err := scanResources(root)
	if err != nil || len(scan.Files) != 1 || scan.Files[0].Name != "driver.inf" {
		t.Fatalf("Finder metadata included: %#v %v", scan, err)
	}
}
