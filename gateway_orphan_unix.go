//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// picoOwnedImageRoots lists the directories this drive's runtime may
// legitimately execute from: the verified local cache and the portable
// runtime directory on the drive.
func picoOwnedImageRoots(root string) []string {
	var bases []string
	if dir, err := runtimeCacheDirectory(); err == nil {
		bases = append(bases, dir)
	}
	bases = append(bases, filepath.Join(root, "runtime", "picoclaw"))
	return bases
}

// picoProcessImagePath returns the full image path of a live process via
// /proc. On platforms without /proc (macOS) this returns false, so the
// orphan-recovery path degrades to the previous manual guidance.
func picoProcessImagePath(pid int) (string, bool) {
	target, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe")
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	return filepath.Clean(target), true
}

// picoProcessIsOurs reports whether the live process at pid runs from one of
// the directories owned by this drive's runtime.
func picoProcessIsOurs(root string, pid int) bool {
	path, ok := picoProcessImagePath(pid)
	if !ok {
		return false
	}
	for _, base := range picoOwnedImageRoots(root) {
		if isPathWithin(base, path) {
			return true
		}
	}
	return false
}

// terminatePicoProcess terminates a pid that already passed ownership
// verification. The signal is sent to the live pid at call time.
func terminatePicoProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
