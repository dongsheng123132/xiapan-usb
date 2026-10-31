package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// workspaceLocks serializes the workspace registry read-modify-write, so
// concurrent workspace.create calls cannot drop each other's registrations.
var workspaceLocks sync.Map

// WorkspaceProject is one registered on-disk project. Path is slash-separated
// and relative to the workspace root (data/pico/workspace), so registrations
// survive a drive-letter or volume-name change.
type WorkspaceProject struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// workspaceRoot resolves and validates the fixed on-disk workspace directory.
// It reuses writablePath so every existing component must be a real, non-link
// directory; read operations never create the directory.
func workspaceRoot(root string) (string, error) {
	return writablePath(root, "data", "pico", "workspace")
}

// cleanRelative validates a user-supplied slash-separated relative path and
// returns a normalized form. It rejects absolute paths, empty/`.`/`..`
// segments, backslashes, drive letters and NUL before any filesystem access.
// An empty input normalizes to "" (the root itself).
func cleanRelative(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", nil
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") || filepath.IsAbs(rel) {
		return "", errors.New("工作区路径必须是盘内相对路径")
	}
	parts := strings.Split(rel, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, "/\\:\x00") {
			return "", errors.New("工作区路径包含无效片段")
		}
		clean = append(clean, p)
	}
	return strings.Join(clean, "/"), nil
}

func parentOf(rel string) string {
	if rel == "" {
		return ""
	}
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}

// mkdirAllWithin creates rel (and any missing parents) inside r using only the
// Go 1.24 os.Root surface. Every existing component must be a real, non-link
// directory, so a symlink in the project path is rejected even when it points
// back inside the root.
func mkdirAllWithin(r *os.Root, rel string, perm os.FileMode) error {
	parts := strings.Split(rel, "/")
	cur := ""
	for _, p := range parts {
		if cur == "" {
			cur = p
		} else {
			cur = cur + "/" + p
		}
		if err := r.Mkdir(cur, perm); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := r.Lstat(cur)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("工作区路径含链接或非目录")
		}
	}
	return nil
}

// loadWorkspaces reads the registered project list. A missing file is an empty
// list and never creates the settings directory or file (read-only on an
// empty disk).
func loadWorkspaces(root string) ([]WorkspaceProject, error) {
	p, err := writablePath(root, "data", "settings", "workspaces.json")
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return []WorkspaceProject{}, nil
	}
	if err != nil {
		return nil, errors.New("无法读取工作区项目列表")
	}
	var list []WorkspaceProject
	if len(b) > 1<<20 || json.Unmarshal(b, &list) != nil {
		return nil, errors.New("工作区项目列表损坏，已保留原文件")
	}
	return list, nil
}

func saveWorkspaces(root string, list []WorkspaceProject) error {
	b, _ := json.MarshalIndent(list, "", "  ")
	return replaceLocalFile(root, []string{"data", "settings", "workspaces.json"}, b)
}

func findWorkspace(list []WorkspaceProject, id string) (WorkspaceProject, bool) {
	for _, p := range list {
		if p.ID == id {
			return p, true
		}
	}
	return WorkspaceProject{}, false
}

func findWorkspaceByPath(list []WorkspaceProject, rel string) (WorkspaceProject, bool) {
	for _, p := range list {
		if p.Path == rel {
			return p, true
		}
	}
	return WorkspaceProject{}, false
}

// workspaceProjectRealPath resolves a registered project to its verified real
// absolute directory, rejecting symlinks and any path that left the workspace
// root. Used to inject a working-directory context into the PicoClaw prompt.
func workspaceProjectRealPath(root, id string) (string, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return "", err
	}
	proj, ok := findWorkspace(list, id)
	if !ok {
		return "", errors.New("工作区项目不存在，请重新选择")
	}
	rel, err := cleanRelative(proj.Path)
	if err != nil || rel == "" {
		return "", errors.New("工作区项目路径无效")
	}
	parts := append([]string{"data", "pico", "workspace"}, strings.Split(rel, "/")...)
	p, err := writablePath(root, parts...)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("工作区项目不是盘内真实目录")
	}
	return filepath.EvalSymlinks(p)
}

func workspaceList(root string) (any, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	rows := make([]any, 0, len(list))
	for _, p := range list {
		rows = append(rows, map[string]any{"id": p.ID, "title": p.Title, "path": p.Path})
	}
	return rows, nil
}

func workspaceBrowse(root, rel string) (any, error) {
	relClean, err := cleanRelative(rel)
	if err != nil {
		return nil, err
	}
	wsRoot, err := workspaceRoot(root)
	if err != nil {
		return nil, err
	}
	empty := func() any {
		return map[string]any{"path": relClean, "parent": parentOf(relClean), "directories": []any{}}
	}
	r, err := os.OpenRoot(wsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty(), nil
		}
		return nil, err
	}
	defer r.Close()
	name := relClean
	if name == "" {
		name = "."
	}
	info, err := r.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("该路径不是工作区内的目录")
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	dirs := make([]any, 0, len(entries))
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 || !e.IsDir() {
			continue
		}
		p := e.Name()
		if relClean != "" {
			p = relClean + "/" + e.Name()
		}
		dirs = append(dirs, map[string]any{"name": e.Name(), "path": p})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].(map[string]any)["name"].(string) < dirs[j].(map[string]any)["name"].(string)
	})
	return map[string]any{"path": relClean, "parent": parentOf(relClean), "directories": dirs}, nil
}

func workspaceCreate(root, rel string) (any, error) {
	relClean, err := cleanRelative(rel)
	if err != nil {
		return nil, err
	}
	if relClean == "" {
		return nil, errors.New("请选择要加入工作区的目录")
	}
	lock := rootLock(&workspaceLocks, root)
	lock.Lock()
	defer lock.Unlock()
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	if existing, ok := findWorkspaceByPath(list, relClean); ok {
		// An idempotent hit must still reference a real, non-link directory:
		// the registered dir may have been replaced by a symlink on disk.
		if _, err := workspaceProjectRealPath(root, existing.ID); err != nil {
			return nil, errors.New("该项目路径已失效，请先移除后重新加入")
		}
		return map[string]any{"id": existing.ID, "title": existing.Title, "path": existing.Path}, nil
	}
	wsRoot, err := workspaceRoot(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(wsRoot, 0700); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(wsRoot)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if err = mkdirAllWithin(r, relClean, 0700); err != nil {
		return nil, err
	}
	proj := WorkspaceProject{
		ID:    uniqueID(),
		Title: filepath.Base(filepath.FromSlash(relClean)),
		Path:  relClean,
	}
	list = append(list, proj)
	if err = saveWorkspaces(root, list); err != nil {
		return nil, err
	}
	return map[string]any{"id": proj.ID, "title": proj.Title, "path": proj.Path}, nil
}

func workspaceFiles(root, id, rel string) (any, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	proj, ok := findWorkspace(list, id)
	if !ok {
		return nil, errors.New("未找到该项目，请先在工作区选择或创建")
	}
	projRel, err := cleanRelative(proj.Path)
	if err != nil || projRel == "" {
		return nil, errors.New("项目路径无效")
	}
	relClean, err := cleanRelative(rel)
	if err != nil {
		return nil, err
	}
	full := projRel
	if relClean != "" {
		full = projRel + "/" + relClean
	}
	wsRoot, err := workspaceRoot(root)
	if err != nil {
		return nil, err
	}
	empty := func() any {
		return map[string]any{"path": relClean, "files": []any{}}
	}
	r, err := os.OpenRoot(wsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty(), nil
		}
		return nil, err
	}
	defer r.Close()
	info, err := r.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("该路径不是项目内的目录")
	}
	f, err := r.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	files := make([]any, 0, len(entries))
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := e.Name()
		if full != "" {
			child = full + "/" + e.Name()
		}
		files = append(files, map[string]any{"name": e.Name(), "path": child, "directory": e.IsDir()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].(map[string]any)["name"].(string) < files[j].(map[string]any)["name"].(string)
	})
	return map[string]any{"path": relClean, "files": files}, nil
}
