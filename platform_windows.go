//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

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
