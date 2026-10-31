package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestWorkspaceRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"..", "../etc", "/absolute", "C:drive", "a\\b", "a/../b", "a//b", ".", "a/."} {
		if r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": p}); r.OK {
			t.Fatalf("browse accepted path %q", p)
		}
		if r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": p}); r.OK {
			t.Fatalf("create accepted path %q", p)
		}
	}
}

func TestWorkspaceRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	wsRoot := filepath.Join(root, "data", "pico", "workspace")
	if err := os.MkdirAll(wsRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(wsRoot, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// Browsing the root must not surface the escaping link as a directory.
	b := runAction(context.Background(), root, "workspace.browse", map[string]any{})
	if !b.OK {
		t.Fatal(b.Error)
	}
	if dirs := b.Data.(map[string]any)["directories"].([]any); len(dirs) != 0 {
		t.Fatalf("symlink surfaced as directory: %#v", dirs)
	}
	// Browsing into the link must fail.
	if r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": "link"}); r.OK {
		t.Fatal("browsed through escaping symlink")
	}
	// Creating under the link must fail and must not write outside.
	if r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "link/sub"}); r.OK {
		t.Fatal("created through escaping symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "sub")); !os.IsNotExist(err) {
		t.Fatal("symlink create wrote outside the workspace")
	}
	// A registered project that contains an escaping link must not leak files.
	cr := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj"})
	if !cr.OK {
		t.Fatal(cr.Error)
	}
	id := cr.Data.(map[string]any)["id"].(string)
	if err := os.Symlink(outside, filepath.Join(wsRoot, "proj", "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	fr := runAction(context.Background(), root, "workspace.files", map[string]any{"workspace_id": id})
	if !fr.OK {
		t.Fatal(fr.Error)
	}
	if files := fr.Data.(map[string]any)["files"].([]any); len(files) != 0 {
		t.Fatalf("escaping link leaked into file list: %#v", files)
	}
}

func TestWorkspaceCreatePersistsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "alpha/beta"})
	if !r.OK {
		t.Fatal(r.Error)
	}
	id := r.Data.(map[string]any)["id"].(string)
	if id == "" {
		t.Fatal("missing project id")
	}
	if _, err := os.Stat(filepath.Join(root, "data", "settings", "workspaces.json")); err != nil {
		t.Fatalf("registration not persisted: %v", err)
	}
	list, err := loadWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != id || list[0].Path != "alpha/beta" || list[0].Title != "beta" {
		t.Fatalf("unexpected registration %#v", list)
	}
	// Re-registering the same path is idempotent.
	r2 := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "alpha/beta"})
	if !r2.OK || r2.Data.(map[string]any)["id"].(string) != id {
		t.Fatalf("duplicate registration: %#v", r2)
	}
	list, _ = loadWorkspaces(root)
	if len(list) != 1 {
		t.Fatalf("duplicate entry persisted: %#v", list)
	}
}

func TestWorkspaceReadsDoNotWriteOnEmptyDisk(t *testing.T) {
	root := t.TempDir()
	if r := runAction(context.Background(), root, "workspace.list", map[string]any{}); !r.OK {
		t.Fatal(r.Error)
	}
	if r := runAction(context.Background(), root, "workspace.browse", map[string]any{}); !r.OK {
		t.Fatal(r.Error)
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatalf("read operations created data directory: %v", err)
	}
}

func TestWorkspaceBrowseAndFilesPaths(t *testing.T) {
	root := t.TempDir()
	cr := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj"})
	if !cr.OK {
		t.Fatal(cr.Error)
	}
	id := cr.Data.(map[string]any)["id"].(string)
	projDir := filepath.Join(root, "data", "pico", "workspace", "proj")
	if err := os.MkdirAll(filepath.Join(projDir, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "a.txt"), []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "docs", "b.txt"), []byte("hi"), 0600); err != nil {
		t.Fatal(err)
	}

	b := runAction(context.Background(), root, "workspace.browse", map[string]any{})
	if !b.OK {
		t.Fatal(b.Error)
	}
	bv := b.Data.(map[string]any)
	if bv["path"] != "" || bv["parent"] != "" {
		t.Fatalf("unexpected root browse %#v", bv)
	}
	dirs := bv["directories"].([]any)
	if len(dirs) != 1 || dirs[0].(map[string]any)["name"] != "proj" || dirs[0].(map[string]any)["path"] != "proj" {
		t.Fatalf("unexpected directories %#v", dirs)
	}

	fr := runAction(context.Background(), root, "workspace.files", map[string]any{"workspace_id": id})
	if !fr.OK {
		t.Fatal(fr.Error)
	}
	fv := fr.Data.(map[string]any)
	files := fv["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("unexpected files %#v", files)
	}
	byName := map[string]map[string]any{}
	for _, f := range files {
		m := f.(map[string]any)
		byName[m["name"].(string)] = m
	}
	if byName["a.txt"]["path"] != "proj/a.txt" || byName["a.txt"]["directory"] != false {
		t.Fatalf("unexpected file entry %#v", byName["a.txt"])
	}
	if byName["docs"]["path"] != "proj/docs" || byName["docs"]["directory"] != true {
		t.Fatalf("unexpected dir entry %#v", byName["docs"])
	}

	sub := runAction(context.Background(), root, "workspace.files", map[string]any{"workspace_id": id, "path": "docs"})
	if !sub.OK {
		t.Fatal(sub.Error)
	}
	subFiles := sub.Data.(map[string]any)["files"].([]any)
	if len(subFiles) != 1 || subFiles[0].(map[string]any)["path"] != "proj/docs/b.txt" {
		t.Fatalf("unexpected subdirectory files %#v", subFiles)
	}
}

func TestChatWorkspaceFilterAndLegacyCompatibility(t *testing.T) {
	root := t.TempDir()
	legacyID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	legacyJSON := `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","title":"旧会话","version":3,"updated_at":"2026-10-31T00:00:00Z","messages":[],"pending":[],"engine":"pico"}`
	if _, err := newFileAtomic(root, []string{"data", "chat", legacyID + ".json"}, []byte(legacyJSON)); err != nil {
		t.Fatal(err)
	}

	cr := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj"})
	if !cr.OK {
		t.Fatal(cr.Error)
	}
	wsID := cr.Data.(map[string]any)["id"].(string)

	sr := runAction(context.Background(), root, "chat.start", map[string]any{"workspace_id": wsID})
	if !sr.OK {
		t.Fatal(sr.Error)
	}

	lr := runAction(context.Background(), root, "chat.list", map[string]any{})
	if !lr.OK {
		t.Fatal(lr.Error)
	}
	rows := lr.Data.([]any)
	if len(rows) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(rows))
	}

	fr := runAction(context.Background(), root, "chat.list", map[string]any{"workspace_id": wsID})
	if !fr.OK {
		t.Fatal(fr.Error)
	}
	filtered := fr.Data.([]any)
	if len(filtered) != 1 || filtered[0].(map[string]any)["workspace_id"] != wsID {
		t.Fatalf("workspace filter returned %#v", filtered)
	}

	s, err := loadChat(root, legacyID)
	if err != nil {
		t.Fatal(err)
	}
	if s.WorkspaceID != "" {
		t.Fatalf("legacy session gained workspace_id: %q", s.WorkspaceID)
	}

	if bad := runAction(context.Background(), root, "chat.start", map[string]any{"workspace_id": "nonexistent"}); bad.OK {
		t.Fatal("accepted unregistered workspace_id")
	}
}

func TestWorkspaceProjectRealPathRejectsEscapingEntry(t *testing.T) {
	root := t.TempDir()
	// A hand-edited registration that points outside must never resolve.
	list := []WorkspaceProject{{ID: "bad", Title: "bad", Path: "../escape"}}
	if err := saveWorkspaces(root, list); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceProjectRealPath(root, "bad"); err == nil {
		t.Fatal("resolved an escaping registration")
	}
}

func TestWorkspaceCreateConcurrentNoLostRegistration(t *testing.T) {
	root := t.TempDir()
	const n = 16
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj" + strconv.Itoa(i)})
			if !r.OK {
				t.Errorf("create %d failed: %s", i, r.Error)
			}
		}(i)
	}
	wg.Wait()
	list, err := loadWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != n {
		t.Fatalf("lost concurrent registrations: got %d want %d", len(list), n)
	}
	seen := map[string]bool{}
	for _, p := range list {
		if seen[p.ID] {
			t.Fatalf("duplicate id %s", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestWorkspaceCreateConcurrentSamePathSingleEntry(t *testing.T) {
	root := t.TempDir()
	const n = 16
	var wg sync.WaitGroup
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "same"})
			if r.OK {
				ids[i] = r.Data.(map[string]any)["id"].(string)
			}
		}(i)
	}
	wg.Wait()
	list, err := loadWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("concurrent same-path create produced %d entries", len(list))
	}
	for _, id := range ids {
		if id != "" && id != list[0].ID {
			t.Fatalf("callers returned divergent ids %q vs %q", id, list[0].ID)
		}
	}
}

func TestWorkspaceCreateIdempotentRejectsReplacedSymlink(t *testing.T) {
	root := t.TempDir()
	first := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj"})
	if !first.OK {
		t.Fatal(first.Error)
	}
	id := first.Data.(map[string]any)["id"].(string)
	projDir := filepath.Join(root, "data", "pico", "workspace", "proj")
	if err := os.RemoveAll(projDir); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, projDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	// Re-registering the same path must not silently accept the replaced link.
	if r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "proj"}); r.OK {
		t.Fatal("accepted a project whose directory was replaced by a symlink")
	}
	// The original registration is untouched.
	list, err := loadWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("registration changed on rejected idempotent hit: %#v", list)
	}
}
