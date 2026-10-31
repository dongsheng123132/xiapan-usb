package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// workspaceLocks serializes the workspace registry read-modify-write, so
// concurrent workspace.create calls cannot drop each other's registrations.
var workspaceLocks sync.Map

// WorkspaceProject is one registered on-disk project. Path is the resolved real
// absolute host directory (backslashes on Windows, forward slashes on
// macOS/Linux), so a project may live anywhere on the computer. After moving to
// another machine the absolute path may no longer exist; workspace.list reports
// that with `available`, and every browse/read reports the missing path instead
// of guessing a replacement.
type WorkspaceProject struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// resolveHostDir validates a user-supplied host path and returns its resolved
// real absolute directory. The directory must already exist: symlinks are
// resolved via EvalSymlinks and the result must be a real, non-link directory.
func resolveHostDir(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("请选择目录")
	}
	if strings.ContainsRune(p, '\x00') {
		return "", errors.New("路径包含无效字符")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("目录不存在")
		}
		return "", err
	}
	info, err := os.Lstat(real)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("路径不是目录")
	}
	return real, nil
}

// createWorkspaceDir normalizes p to a resolved real absolute directory,
// creating it (and any missing parents) when it does not exist yet.
func createWorkspaceDir(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("请选择要加入工作区的目录")
	}
	if strings.ContainsRune(p, '\x00') {
		return "", errors.New("路径包含无效字符")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return "", errors.New("无法创建该目录（路径不是目录或无权限）")
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(real)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("路径不是目录")
	}
	return real, nil
}

// workspaceDirAvailable reports whether an absolute host path currently exists
// as a real, non-link directory on this machine.
func workspaceDirAvailable(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink == 0 && info.IsDir()
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

func findWorkspaceByPath(list []WorkspaceProject, abs string) (WorkspaceProject, bool) {
	for _, p := range list {
		if p.Path == abs {
			return p, true
		}
	}
	return WorkspaceProject{}, false
}

// workspaceProjectRealPath validates that a registered project's absolute path
// still exists as a real, non-link directory on this machine and returns it.
// Used to inject a working-directory context into the PicoClaw prompt.
func workspaceProjectRealPath(root, id string) (string, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return "", err
	}
	proj, ok := findWorkspace(list, id)
	if !ok {
		return "", errors.New("工作区项目不存在，请重新选择")
	}
	if !filepath.IsAbs(proj.Path) || !workspaceDirAvailable(proj.Path) {
		return "", errors.New("项目路径在本机不存在，请重新选择")
	}
	return proj.Path, nil
}

func workspaceList(root string) (any, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	rows := make([]any, 0, len(list))
	for _, p := range list {
		rows = append(rows, map[string]any{
			"id":        p.ID,
			"title":     filepath.Base(p.Path),
			"path":      p.Path,
			"available": workspaceDirAvailable(p.Path),
		})
	}
	return rows, nil
}

// workspaceBrowse lists the subdirectories of an absolute host directory, one
// level deep. An empty path starts from the user's home directory. Only real,
// non-link directories are returned; each child path is absolute.
func workspaceBrowse(root, path string) (any, error) {
	if strings.TrimSpace(path) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = home
	}
	dir, err := resolveHostDir(path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	dirs := make([]any, 0, len(entries))
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 || !e.IsDir() {
			continue
		}
		dirs = append(dirs, map[string]any{"name": e.Name(), "path": filepath.Join(dir, e.Name())})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].(map[string]any)["name"].(string) < dirs[j].(map[string]any)["name"].(string)
	})
	return map[string]any{"path": dir, "parent": filepath.Dir(dir), "directories": dirs}, nil
}

func workspaceCreate(root, path string) (any, error) {
	dir, err := createWorkspaceDir(path)
	if err != nil {
		return nil, err
	}
	lock := rootLock(&workspaceLocks, root)
	lock.Lock()
	defer lock.Unlock()
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	if existing, ok := findWorkspaceByPath(list, dir); ok {
		// An idempotent hit must still reference a real, non-link directory:
		// the registered dir may have been replaced by a symlink on disk.
		if _, err := workspaceProjectRealPath(root, existing.ID); err != nil {
			return nil, errors.New("该项目路径已失效，请先移除后重新加入")
		}
		return map[string]any{"id": existing.ID, "title": filepath.Base(dir), "path": dir}, nil
	}
	proj := WorkspaceProject{
		ID:    uniqueID(),
		Title: filepath.Base(dir),
		Path:  dir,
	}
	list = append(list, proj)
	if err = saveWorkspaces(root, list); err != nil {
		return nil, err
	}
	return map[string]any{"id": proj.ID, "title": proj.Title, "path": proj.Path}, nil
}

// workspaceFiles lists the files and subdirectories of a project directory, one
// level deep. An empty path means the project root; a non-empty path is an
// absolute subdirectory path. Directories are listed first, then by name.
func workspaceFiles(root, id, path string) (any, error) {
	list, err := loadWorkspaces(root)
	if err != nil {
		return nil, err
	}
	proj, ok := findWorkspace(list, id)
	if !ok {
		return nil, errors.New("未找到该项目，请先在工作区选择或创建")
	}
	target := proj.Path
	if strings.TrimSpace(path) != "" {
		target = path
	}
	dir, err := resolveHostDir(target)
	if err != nil {
		if strings.TrimSpace(path) == "" {
			return nil, errors.New("项目路径在本机不存在，请重新选择")
		}
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]any, 0, len(entries))
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		row := map[string]any{
			"name":      e.Name(),
			"path":      filepath.Join(dir, e.Name()),
			"directory": e.IsDir(),
		}
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				row["size_bytes"] = info.Size()
			}
		}
		files = append(files, row)
	}
	sort.Slice(files, func(i, j int) bool {
		a := files[i].(map[string]any)
		b := files[j].(map[string]any)
		if a["directory"].(bool) != b["directory"].(bool) {
			return a["directory"].(bool)
		}
		return a["name"].(string) < b["name"].(string)
	})
	return map[string]any{"path": dir, "files": files}, nil
}
