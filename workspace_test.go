package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func resolveForTest(t *testing.T, p string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return real
}

func TestWorkspaceCreatePersistsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "alpha", "beta")
	r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": target})
	if !r.OK {
		t.Fatal(r.Error)
	}
	got := r.Data.(map[string]any)
	id := got["id"].(string)
	if id == "" {
		t.Fatal("missing project id")
	}
	real := resolveForTest(t, target)
	if got["path"] != real {
		t.Fatalf("path %q want %q", got["path"], real)
	}
	if got["title"] != "beta" {
		t.Fatalf("title %#v want %q", got["title"], "beta")
	}
	if _, err := os.Stat(filepath.Join(root, "data", "settings", "workspaces.json")); err != nil {
		t.Fatalf("registration not persisted: %v", err)
	}
	list, err := loadWorkspaces(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != id || list[0].Path != real || list[0].Title != "beta" {
		t.Fatalf("unexpected registration %#v", list)
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		t.Fatalf("project dir was not created on disk: %v", err)
	}
	// Re-registering the same absolute path is idempotent.
	r2 := runAction(context.Background(), root, "workspace.create", map[string]any{"path": target})
	if !r2.OK || r2.Data.(map[string]any)["id"].(string) != id {
		t.Fatalf("duplicate registration: %#v", r2)
	}
	list, _ = loadWorkspaces(root)
	if len(list) != 1 {
		t.Fatalf("duplicate entry persisted: %#v", list)
	}
}

func TestWorkspaceBrowseListsSubdirectories(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "alpha"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "beta"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "note.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": base})
	if !r.OK {
		t.Fatal(r.Error)
	}
	b := r.Data.(map[string]any)
	real := resolveForTest(t, base)
	if b["path"] != real {
		t.Fatalf("path %q want %q", b["path"], real)
	}
	if b["parent"] != filepath.Dir(real) {
		t.Fatalf("parent %q want %q", b["parent"], filepath.Dir(real))
	}
	dirs := b["directories"].([]any)
	if len(dirs) != 2 {
		t.Fatalf("unexpected directories %#v", dirs)
	}
	names := make([]string, 0, len(dirs))
	for _, d := range dirs {
		m := d.(map[string]any)
		names = append(names, m["name"].(string))
		if m["path"] != filepath.Join(real, m["name"].(string)) {
			t.Fatalf("directory path mismatch %#v", m)
		}
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("directories not sorted: %v", names)
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("unexpected order %v", names)
	}
	for _, d := range dirs {
		if d.(map[string]any)["name"] == "note.txt" {
			t.Fatal("regular file surfaced in browse")
		}
	}
}

func TestWorkspaceBrowseEmptyStartsAtHome(t *testing.T) {
	root := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	r := runAction(context.Background(), root, "workspace.browse", map[string]any{})
	if !r.OK {
		t.Fatal(r.Error)
	}
	if got := r.Data.(map[string]any)["path"].(string); got != resolveForTest(t, home) {
		t.Fatalf("browse empty path %q want home %q", got, resolveForTest(t, home))
	}
}

func TestWorkspaceFilesListsFilesAndSizes(t *testing.T) {
	root := t.TempDir()
	cr := runAction(context.Background(), root, "workspace.create", map[string]any{"path": filepath.Join(t.TempDir(), "proj")})
	if !cr.OK {
		t.Fatal(cr.Error)
	}
	id := cr.Data.(map[string]any)["id"].(string)
	projPath := cr.Data.(map[string]any)["path"].(string)

	if err := os.MkdirAll(filepath.Join(projPath, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projPath, "a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projPath, "docs", "b.txt"), []byte("world!"), 0600); err != nil {
		t.Fatal(err)
	}

	fr := runAction(context.Background(), root, "workspace.files", map[string]any{"workspace_id": id})
	if !fr.OK {
		t.Fatal(fr.Error)
	}
	fv := fr.Data.(map[string]any)
	if fv["path"] != projPath {
		t.Fatalf("files path %#v want %q", fv["path"], projPath)
	}
	files := fv["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("unexpected files %#v", files)
	}
	// Directories are listed first, then by name: docs before a.txt.
	docs := files[0].(map[string]any)
	if docs["name"] != "docs" || docs["path"] != filepath.Join(projPath, "docs") || docs["directory"] != true {
		t.Fatalf("unexpected docs entry %#v", docs)
	}
	a := files[1].(map[string]any)
	if a["name"] != "a.txt" || a["path"] != filepath.Join(projPath, "a.txt") || a["directory"] != false {
		t.Fatalf("unexpected a.txt entry %#v", a)
	}
	if a["size_bytes"].(int64) != 5 {
		t.Fatalf("a.txt size_bytes %#v want 5", a["size_bytes"])
	}

	sub := runAction(context.Background(), root, "workspace.files", map[string]any{"workspace_id": id, "path": filepath.Join(projPath, "docs")})
	if !sub.OK {
		t.Fatal(sub.Error)
	}
	subFiles := sub.Data.(map[string]any)["files"].([]any)
	if len(subFiles) != 1 || subFiles[0].(map[string]any)["path"] != filepath.Join(projPath, "docs", "b.txt") {
		t.Fatalf("unexpected subdirectory files %#v", subFiles)
	}
	if subFiles[0].(map[string]any)["size_bytes"].(int64) != 6 {
		t.Fatalf("b.txt size_bytes %#v want 6", subFiles[0].(map[string]any)["size_bytes"])
	}
}

func TestWorkspaceListAvailableAndTitle(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(t.TempDir(), "docs")
	if err := os.MkdirAll(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	realPath := resolveForTest(t, realDir)
	missing := filepath.Join(t.TempDir(), "gone")
	list := []WorkspaceProject{
		{ID: "real", Path: realPath},
		{ID: "gone", Path: missing},
	}
	if err := saveWorkspaces(root, list); err != nil {
		t.Fatal(err)
	}
	r := runAction(context.Background(), root, "workspace.list", map[string]any{})
	if !r.OK {
		t.Fatal(r.Error)
	}
	rows := r.Data.([]any)
	if len(rows) != 2 {
		t.Fatalf("unexpected rows %#v", rows)
	}
	first := rows[0].(map[string]any)
	if first["id"] != "real" || first["title"] != "docs" || first["path"] != realPath || first["available"] != true {
		t.Fatalf("unexpected first row %#v", first)
	}
	second := rows[1].(map[string]any)
	if second["id"] != "gone" || second["title"] != "gone" || second["path"] != missing || second["available"] != false {
		t.Fatalf("unexpected second row %#v", second)
	}
}

func TestWorkspaceProjectRealPathRejectsRelativeAndMissing(t *testing.T) {
	root := t.TempDir()
	// A legacy relative-path registration is not valid in the new semantics.
	list := []WorkspaceProject{{ID: "rel", Path: "alpha/beta"}}
	if err := saveWorkspaces(root, list); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceProjectRealPath(root, "rel"); err == nil {
		t.Fatal("resolved a relative-path registration")
	}
	// An absolute path that no longer exists is reported clearly.
	missing := filepath.Join(t.TempDir(), "gone")
	list = []WorkspaceProject{{ID: "gone", Path: missing}}
	if err := saveWorkspaces(root, list); err != nil {
		t.Fatal(err)
	}
	_, err := workspaceProjectRealPath(root, "gone")
	if err == nil || !strings.Contains(err.Error(), "项目路径在本机不存在") {
		t.Fatalf("unexpected missing-path error %v", err)
	}
}

func TestWorkspaceBrowseMissingPathError(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": missing})
	if r.OK {
		t.Fatal("browse accepted a missing path")
	}
	if !strings.Contains(r.Error, "目录不存在") {
		t.Fatalf("unexpected error %q", r.Error)
	}
}

func TestWorkspaceCreateRejectsInvalidPath(t *testing.T) {
	root := t.TempDir()
	if r := runAction(context.Background(), root, "workspace.create", map[string]any{}); r.OK {
		t.Fatal("create accepted empty path")
	}
	if r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": "bad\x00path"}); r.OK {
		t.Fatal("create accepted NUL path")
	}
	file := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": file}); r.OK {
		t.Fatal("create accepted a regular file path")
	}
}

func TestWorkspaceBrowseRejectsInvalidPath(t *testing.T) {
	root := t.TempDir()
	if r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": "bad\x00path"}); r.OK {
		t.Fatal("browse accepted NUL path")
	}
	file := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := runAction(context.Background(), root, "workspace.browse", map[string]any{"path": file}); r.OK {
		t.Fatal("browse accepted a regular file path")
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

func TestWorkspaceCreateConcurrentNoLostRegistration(t *testing.T) {
	root := t.TempDir()
	base := t.TempDir()
	const n = 16
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": filepath.Join(base, "proj"+strconv.Itoa(i))})
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
	target := filepath.Join(t.TempDir(), "same")
	const n = 16
	var wg sync.WaitGroup
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": target})
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

func TestChatWorkspaceFilterAndLegacyCompatibility(t *testing.T) {
	root := t.TempDir()
	legacyID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	legacyJSON := `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","title":"旧会话","version":3,"updated_at":"2026-10-31T00:00:00Z","messages":[],"pending":[],"engine":"pico"}`
	if _, err := newFileAtomic(root, []string{"data", "chat", legacyID + ".json"}, []byte(legacyJSON)); err != nil {
		t.Fatal(err)
	}

	cr := runAction(context.Background(), root, "workspace.create", map[string]any{"path": filepath.Join(t.TempDir(), "proj")})
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

func TestWriteProjectGuideIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data", "pico", "workspace"), 0700); err != nil {
		t.Fatal(err)
	}
	first := "/Users/example/first"
	second := "/Users/example/second"

	if err := writeProjectGuide(root, first); err != nil {
		t.Fatal(err)
	}
	// Pre-existing unrelated content must survive the marked-section replacement.
	guide := filepath.Join(root, "data", "pico", "workspace", "AGENTS.md")
	if err := replaceLocalFile(root, []string{"data", "pico", "workspace", "AGENTS.md"}, []byte("# 我的规则\n保留这一行\n\n"+mustRead(t, guide))); err != nil {
		t.Fatal(err)
	}
	if err := writeProjectGuide(root, second); err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, guide)
	if !strings.Contains(body, "我的规则") || !strings.Contains(body, "保留这一行") {
		t.Fatalf("unrelated content was dropped:\n%s", body)
	}
	if !strings.Contains(body, second) {
		t.Fatalf("second path missing:\n%s", body)
	}
	if strings.Contains(body, first) {
		t.Fatalf("first path was not replaced:\n%s", body)
	}
	if strings.Count(body, projectGuideStart) != 1 {
		t.Fatalf("section marker duplicated:\n%s", body)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
