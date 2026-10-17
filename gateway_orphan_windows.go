//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var picoQueryFullProcessImageName = picoKernel32.NewProc("QueryFullProcessImageNameW")

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

// picoProcessImagePath returns the full image path of a live process. The
// path is compared as a path prefix, never used as a kill authority by itself.
func picoProcessImagePath(pid int) (string, bool) {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return "", false
	}
	defer syscall.CloseHandle(handle)
	buf := make([]uint16, 4096)
	size := uint32(len(buf))
	r1, _, _ := picoQueryFullProcessImageName.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 || size == 0 {
		return "", false
	}
	path := syscall.UTF16ToString(buf[:size])
	if path == "" {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path), true
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
// verification. The handle is opened from the live pid at call time, so a
// process that exited or was reused between verification and termination
// fails closed (OpenProcess refuses a dead pid).
func terminatePicoProcess(pid int) error {
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	return syscall.TerminateProcess(handle, 1)
}
