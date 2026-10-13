package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Only reproducible, manifest-verified PicoClaw programs belong here. Portable
// config, credentials, conversations and workspace files always stay on the USB.
const runtimeCacheLimit = int64(256 << 20)

var runtimeCacheLocks sync.Map
var runtimeCacheStates sync.Map

type runtimeCacheState struct {
	Status string
	Path   string
	Reason string
}

// runtimeCacheInfo is read-only: opening the home page neither copies programs
// nor creates cache directories. The first AI launch prepares its own cache.
func runtimeCacheInfo(root string) map[string]any {
	dir, err := runtimeCacheDirectory()
	result := map[string]any{
		"directory": dir, "platform": runtime.GOOS + "/" + runtime.GOARCH,
		"scope":  "仅缓存已校验的 PicoClaw 程序；Key、钱包、会话与工作文件仍随盘保存",
		"status": "not_used", "lazy": true, "fallback": "verified_usb_program",
	}
	if err != nil {
		result["status"], result["reason"] = "unavailable", err.Error()
	}
	if state, ok := runtimeCacheStates.Load(root); ok {
		s := state.(runtimeCacheState)
		result["status"], result["program"], result["reason"] = s.Status, s.Path, s.Reason
	}
	return result
}

func runtimeCacheDirectory() (string, error) {
	// This override is also useful for disposable test roots. It changes only
	// this application's cache; it never changes HOME, TEMP or system settings.
	if override := os.Getenv("XIAPAN_RUNTIME_CACHE_DIR"); override != "" {
		if !filepath.IsAbs(override) {
			return "", errors.New("程序缓存目录必须是绝对路径")
		}
		return filepath.Clean(override), nil
	}
	base, err := os.UserCacheDir() // LocalAppData on Windows, Library/Caches on macOS.
	if err != nil || !filepath.IsAbs(base) {
		return "", errors.New("无法定位本机程序缓存目录")
	}
	return filepath.Join(base, "Xiapan", "picoclaw-runtime-v1"), nil
}

func cacheResourceArguments(relativePath, expected string) (string, string, error) {
	if strings.ContainsAny(relativePath, "\\:\x00") || filepath.IsAbs(relativePath) {
		return "", "", errors.New("无效的程序资源路径")
	}
	for _, part := range strings.Split(relativePath, "/") {
		if part == "" || part == "." || part == ".." {
			return "", "", errors.New("无效的程序资源路径")
		}
	}
	rel := strings.TrimPrefix(relativePath, "app/")
	if !strings.HasPrefix(rel, "runtime/picoclaw/") {
		return "", "", errors.New("本机缓存仅接收固定清单中的 PicoClaw 程序")
	}
	expected = strings.ToLower(expected)
	digest, err := hex.DecodeString(expected)
	if err != nil || len(digest) != sha256.Size {
		return "", "", errors.New("程序必须提供完整 SHA-256")
	}
	return rel, expected, nil
}

// cacheRuntimeResource returns a verified local executable when available. A
// cache failure is non-fatal: only a separately verified USB source may be used
// as fallback. A hit deliberately never stats or reads the source USB program.
func cacheRuntimeResource(root, relativePath, expected string) (string, error) {
	rel, expected, err := cacheResourceArguments(relativePath, expected)
	if err != nil {
		return "", err
	}
	cacheDir, cacheErr := runtimeCacheDirectory()
	if cacheErr == nil {
		path, status, err := cachedRuntime(root, rel, expected, cacheDir)
		if err == nil {
			runtimeCacheStates.Store(root, runtimeCacheState{Status: status, Path: path})
			return path, nil
		}
		cacheErr = err
	}
	path, sourceErr := verifiedCacheSource(root, rel, expected)
	if sourceErr != nil {
		runtimeCacheStates.Store(root, runtimeCacheState{Status: "unavailable", Reason: cacheErr.Error()})
		return "", fmt.Errorf("程序缓存不可用，盘内程序也未通过校验：%w", sourceErr)
	}
	runtimeCacheStates.Store(root, runtimeCacheState{Status: "usb_fallback", Path: path, Reason: cacheErr.Error()})
	return path, nil
}

func cachedRuntime(root, rel, expected, cacheDir string) (string, string, error) {
	platformDir := filepath.Join(cacheDir, runtime.GOOS+"-"+runtime.GOARCH)
	entryDir := filepath.Join(platformDir, expected)
	program := filepath.Join(entryDir, filepath.Base(rel))
	lockValue, _ := runtimeCacheLocks.LoadOrStore(program, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if err := prepareRuntimeCacheDirs(cacheDir, platformDir, entryDir); err != nil {
		return "", "", err
	}
	if err := verifyCachedProgram(program, expected); err == nil {
		return program, "hit", nil
	}
	// A directory lock prevents two application processes copying the same USB
	// executable at once. A stale lock is never deleted speculatively; after a
	// bounded wait the caller falls back to its verified source.
	lockDir := filepath.Join(entryDir, ".copy-lock")
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.Mkdir(lockDir, 0700)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) || time.Now().After(deadline) {
			return "", "", errors.New("本机程序缓存暂时不可写或正在准备")
		}
		if err := verifyCachedProgram(program, expected); err == nil {
			return program, "hit", nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	defer os.Remove(lockDir)
	if err := verifyCachedProgram(program, expected); err == nil {
		return program, "hit", nil
	}
	if info, err := os.Lstat(program); err == nil && !info.Mode().IsRegular() {
		return "", "", errors.New("程序缓存入口不是普通文件")
	}
	source, _, err := openCacheSource(root, rel)
	if err != nil {
		return "", "", err
	}
	defer source.Close()
	tmp, err := os.CreateTemp(entryDir, ".program-*.tmp")
	if err != nil {
		return "", "", err
	}
	tempName := tmp.Name()
	defer os.Remove(tempName)
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(source, runtimeCacheLimit+1))
	if copyErr == nil && (n > runtimeCacheLimit || hex.EncodeToString(hash.Sum(nil)) != expected) {
		copyErr = errors.New("盘内程序校验不通过")
	}
	if copyErr == nil {
		copyErr = tmp.Chmod(0700)
	}
	if copyErr == nil {
		copyErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", "", copyErr
	}
	if closeErr != nil {
		return "", "", closeErr
	}
	// Recheck directory components before publishing. No partial executable is
	// ever visible under its final name, including if a copy is interrupted.
	if err := prepareRuntimeCacheDirs(cacheDir, platformDir, entryDir); err != nil {
		return "", "", err
	}
	if err := os.Rename(tempName, program); err != nil {
		return "", "", err
	}
	if err := verifyCachedProgram(program, expected); err != nil {
		return "", "", err
	}
	return program, "copied", nil
}

func prepareRuntimeCacheDirs(dirs ...string) error {
	for _, dir := range dirs {
		// Check every component of our destination. Existing user directories
		// are never chmod'ed or cleared. The filesystem root itself is trusted.
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		volume := filepath.VolumeName(abs)
		current := volume + string(filepath.Separator)
		for _, part := range strings.Split(strings.TrimPrefix(abs, current), string(filepath.Separator)) {
			if part == "" {
				continue
			}
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if errors.Is(err, os.ErrNotExist) {
				if err = os.Mkdir(current, 0700); err != nil && !errors.Is(err, os.ErrExist) {
					return err
				}
				info, err = os.Lstat(current)
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return errors.New("程序缓存路径含链接或非目录，使用盘内校验程序")
			}
		}
	}
	return nil
}

func openCacheSource(root, rel string) (*os.File, string, error) {
	path, err := runtimeResourcePath(root, rel)
	if err != nil {
		return nil, "", err
	}
	inside, err := filepath.Rel(root, path)
	if err != nil {
		return nil, "", err
	}
	// Root keeps a concurrent symlink replacement from escaping the USB root.
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()
	f, err := dir.Open(inside)
	if err != nil {
		return nil, "", err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > runtimeCacheLimit {
		f.Close()
		return nil, "", errors.New("盘内程序不是有效的普通文件")
	}
	return f, path, nil
}

func verifiedCacheSource(root, rel, expected string) (string, error) {
	f, path, err := openCacheSource(root, rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := checkRuntimeDigest(f, expected); err != nil {
		return "", err
	}
	return path, nil
}

func verifyCachedProgram(path, expected string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > runtimeCacheLimit {
		return errors.New("程序缓存不是有效的普通文件")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0100 == 0 {
		return errors.New("程序缓存没有执行权限")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return checkRuntimeDigest(f, expected)
}

func checkRuntimeDigest(f *os.File, expected string) error {
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, runtimeCacheLimit+1))
	if err != nil {
		return err
	}
	if n > runtimeCacheLimit || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("程序 SHA-256 校验不通过")
	}
	return nil
}
