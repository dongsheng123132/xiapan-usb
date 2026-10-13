package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func cacheTestDigest(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func cacheTestFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	// macOS' temporary directory may be exposed through /var -> /private/var.
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "中文 USB")
	rel := "runtime/picoclaw/test/picoclaw"
	if runtime.GOOS == "windows" {
		rel += ".exe"
	}
	source := filepath.Join(root, "app", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte(strings.Repeat("public executable fixture\n", 4096))
	if err := os.WriteFile(source, body, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XIAPAN_RUNTIME_CACHE_DIR", filepath.Join(base, "host-cache"))
	return root, rel, source, cacheTestDigest(body)
}

func TestRuntimeCacheHitDoesNotReadUSB(t *testing.T) {
	root, rel, source, hash := cacheTestFixture(t)
	program, err := cacheRuntimeResource(root, rel, hash)
	if err != nil || program == source {
		t.Fatalf("cache miss not copied: %q %v", program, err)
	}
	if !strings.Contains(program, filepath.Join(runtime.GOOS+"-"+runtime.GOARCH, hash)) {
		t.Fatalf("platform and hash are not isolated: %s", program)
	}
	if runtimeCacheInfo(root)["status"] != "copied" {
		t.Fatal("copy status missing")
	}
	// Deleting the source is stronger than an I/O timing assertion: any open or
	// stat of the USB binary on a warm cache would now fail.
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	hit, err := cacheRuntimeResource(root, "app/"+rel, hash)
	if err != nil || hit != program || runtimeCacheInfo(root)["status"] != "hit" {
		t.Fatalf("cache hit depended on source: %s %v", hit, err)
	}
	if _, err := cacheRuntimeResource(root, rel, cacheTestDigest([]byte("different version"))); err == nil {
		t.Fatal("unrelated version reused old cache")
	}
}

func TestRuntimeCacheVersionIsolationAndRepair(t *testing.T) {
	root, rel, source, oldHash := cacheTestFixture(t)
	oldProgram, err := cacheRuntimeResource(root, rel, oldHash)
	if err != nil {
		t.Fatal(err)
	}
	newBody := []byte("second fixed release")
	newHash := cacheTestDigest(newBody)
	if err := os.WriteFile(source, newBody, 0700); err != nil {
		t.Fatal(err)
	}
	newProgram, err := cacheRuntimeResource(root, rel, newHash)
	if err != nil || newProgram == oldProgram {
		t.Fatalf("new hash not isolated: %s %v", newProgram, err)
	}
	if err := os.WriteFile(newProgram, []byte("corrupted local cache"), 0700); err != nil {
		t.Fatal(err)
	}
	repaired, err := cacheRuntimeResource(root, rel, newHash)
	if err != nil || repaired != newProgram {
		t.Fatalf("cache corruption not repaired: %s %v", repaired, err)
	}
	if body, err := os.ReadFile(repaired); err != nil || string(body) != string(newBody) {
		t.Fatalf("invalid repaired program: %q %v", body, err)
	}
	if old, err := cacheRuntimeResource(root, rel, oldHash); err != nil || old != oldProgram {
		t.Fatalf("old version unexpectedly lost: %s %v", old, err)
	}
	if err := os.WriteFile(newProgram, []byte("corrupt cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("corrupt USB"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := cacheRuntimeResource(root, rel, newHash); err == nil {
		t.Fatal("returned an unverified program when both copies were corrupt")
	}
}

func TestRuntimeCacheConcurrentPublication(t *testing.T) {
	root, rel, _, hash := cacheTestFixture(t)
	const workers = 12
	paths, errs := make([]string, workers), make([]error, workers)
	var work sync.WaitGroup
	for i := range paths {
		work.Add(1)
		go func(i int) {
			defer work.Done()
			paths[i], errs[i] = cacheRuntimeResource(root, rel, hash)
		}(i)
	}
	work.Wait()
	for i := range paths {
		if errs[i] != nil || paths[i] != paths[0] {
			t.Fatalf("caller %d observed a partial copy: %s %v", i, paths[i], errs[i])
		}
	}
	entries, err := os.ReadDir(filepath.Dir(paths[0]))
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(paths[0]) {
		t.Fatalf("temporary file or copy lock leaked: %v %v", entries, err)
	}
}

func TestRuntimeCacheProcessHelper(t *testing.T) {
	root := os.Getenv("XIAPAN_TEST_CACHE_SOURCE")
	if root == "" {
		return
	}
	path, err := cacheRuntimeResource(root, os.Getenv("XIAPAN_TEST_CACHE_RESOURCE"), os.Getenv("XIAPAN_TEST_CACHE_HASH"))
	if err != nil || runtimeCacheInfo(root)["status"] == "usb_fallback" {
		t.Fatalf("separate process did not use verified cache: %s %v", path, err)
	}
	fmt.Fprintln(os.Stdout, path)
}

func TestRuntimeCacheConcurrentProcesses(t *testing.T) {
	root, rel, _, hash := cacheTestFixture(t)
	const processes = 4
	outputs, errs := make([][]byte, processes), make([]error, processes)
	var work sync.WaitGroup
	for i := range outputs {
		work.Add(1)
		go func(i int) {
			defer work.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestRuntimeCacheProcessHelper$")
			cmd.Env = append(os.Environ(), "XIAPAN_TEST_CACHE_SOURCE="+root,
				"XIAPAN_TEST_CACHE_RESOURCE="+rel, "XIAPAN_TEST_CACHE_HASH="+hash)
			outputs[i], errs[i] = cmd.CombinedOutput()
		}(i)
	}
	work.Wait()
	for i := range outputs {
		if errs[i] != nil || string(outputs[i]) != string(outputs[0]) {
			t.Fatalf("process %d: %s %v; first: %s", i, outputs[i], errs[i], outputs[0])
		}
	}
	path := strings.Split(strings.TrimSpace(string(outputs[0])), "\n")[0]
	if err := verifyCachedProgram(strings.TrimSpace(path), hash); err != nil {
		t.Fatalf("published result is not a complete executable: %v", err)
	}
}

func TestRuntimeCacheFallbackAndScope(t *testing.T) {
	root, rel, source, hash := cacheTestFixture(t)
	blocked := filepath.Join(t.TempDir(), "blocked-cache")
	if err := os.WriteFile(blocked, []byte("keep existing user file"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XIAPAN_RUNTIME_CACHE_DIR", blocked)
	path, err := cacheRuntimeResource(root, rel, hash)
	if err != nil || path != source || runtimeCacheInfo(root)["status"] != "usb_fallback" {
		t.Fatalf("unavailable cache blocked verified source: %s %v", path, err)
	}
	if body, _ := os.ReadFile(blocked); string(body) != "keep existing user file" {
		t.Fatal("existing cache blocker was modified")
	}
	for _, invalid := range []string{
		"../runtime/picoclaw/test/program", "runtime/picoclaw/../secret", "runtime//picoclaw/program",
		"data/picoclaw/config.json", "app/data/wallet.json", "runtime\\picoclaw\\program.exe", "C:/program.exe",
	} {
		if _, err := cacheRuntimeResource(root, invalid, hash); err == nil {
			t.Errorf("invalid/non-program resource accepted: %s", invalid)
		}
	}
	if _, err := cacheRuntimeResource(root, rel, "missing-hash"); err == nil {
		t.Fatal("unverified resource accepted")
	}
	if _, err := cacheRuntimeResource(root, rel, cacheTestDigest([]byte("wrong hash"))); err == nil {
		t.Fatal("fallback ignored integrity check")
	}
}

func TestRuntimeCacheInfoIsReadOnly(t *testing.T) {
	root, _, _, _ := cacheTestFixture(t)
	dir := os.Getenv("XIAPAN_RUNTIME_CACHE_DIR")
	info := runtimeCacheInfo(root)
	if info["directory"] != dir || info["status"] != "not_used" || info["lazy"] != true {
		t.Fatalf("unexpected cache info: %#v", info)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("opening the home page created a cache")
	}
	t.Setenv("XIAPAN_RUNTIME_CACHE_DIR", "")
	base, err := os.UserCacheDir()
	if err != nil {
		t.Skip("OS does not expose a cache directory")
	}
	actual, err := runtimeCacheDirectory()
	if err != nil || actual != filepath.Join(base, "Xiapan", "picoclaw-runtime-v1") {
		t.Fatalf("platform cache directory mismatch: %s %v", actual, err)
	}
}

func TestRuntimeCacheRejectsLinks(t *testing.T) {
	root, rel, source, hash := cacheTestFixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := os.Getenv("XIAPAN_RUNTIME_CACHE_DIR")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not available to test account: %v", err)
	}
	path, err := cacheRuntimeResource(root, rel, hash)
	if err != nil || path != source {
		t.Fatalf("unsafe cache did not fall back: %s %v", path, err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cache wrote through symlink: %v %v", entries, err)
	}
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	outfile := filepath.Join(outside, "program")
	if err := os.WriteFile(outfile, body, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outfile, source); err != nil {
		t.Fatal(err)
	}
	if _, err := cacheRuntimeResource(root, rel, hash); err == nil {
		t.Fatal("source symlink escaped the USB resource root")
	}
}
