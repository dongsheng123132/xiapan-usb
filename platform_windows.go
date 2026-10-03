//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Only a tool already selected by the user may reach this fallback. Elevate
// that native tool, never the whole assistant or arbitrary model commands.
func launchElevatedTool(ctx context.Context, program string, args []string, cause error) (int, error) {
	if !errors.Is(cause, syscall.Errno(740)) {
		return 0, cause
	}
	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		verb, _ := syscall.UTF16PtrFromString("runas")
		file, _ := syscall.UTF16PtrFromString(program)
		quoted := []string{}
		for _, a := range args {
			quoted = append(quoted, syscall.EscapeArg(a))
		}
		parameters, _ := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
		code, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(parameters)), 0, 1)
		if code <= 32 {
			result <- fmt.Errorf("Windows 未完成管理员授权或启动（错误码 %d）", code)
		} else {
			result <- nil
		}
	}()
	select {
	case err := <-result:
		return 0, err
	case <-deadline.Done():
		return 0, errors.New("等待 Windows 授权超时；请检查或取消系统提示，勿重复启动")
	}
}

func quietProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} }

func nativeSystemDir() string {
	var buffer [32768]uint16
	n, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 || n >= uintptr(len(buffer)) {
		return ""
	}
	return syscall.UTF16ToString(buffer[:n])
}

func platformInfo(root string) (uint64, uint64, []Volume, []string) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	var status struct {
		Length               uint32
		Load                 uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	kernel.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&status)))
	mask, _, _ := kernel.NewProc("GetLogicalDrives").Call()
	volumes := []Volume{}
	notes := []string{}
	for n := 0; n < 26; n++ {
		if mask&(1<<n) == 0 {
			continue
		}
		path := string(rune('A'+n)) + ":\\"
		p, _ := syscall.UTF16PtrFromString(path)
		kind, _, _ := kernel.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(p)))
		if kind != 2 && kind != 3 {
			continue
		}
		var free, total, totalFree uint64
		ok, _, _ := kernel.NewProc("GetDiskFreeSpaceExW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree)))
		if ok == 0 {
			continue
		}
		var label, format [261]uint16
		kernel.NewProc("GetVolumeInformationW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&label[0])), 261, 0, 0, 0, uintptr(unsafe.Pointer(&format[0])), 261)
		v := Volume{path, syscall.UTF16ToString(label[:]), syscall.UTF16ToString(format[:]), kind == 2, total, free}
		volumes = append(volumes, v)
		if v.Removable && strings.EqualFold(v.FileSystem, "NTFS") {
			notes = append(notes, "检测到 NTFS 移动盘；跨平台写入能力需要单独验证，不能由 Windows 测试推断。")
		}
	}
	return status.TotalPhys, status.AvailPhys, volumes, notes
}
