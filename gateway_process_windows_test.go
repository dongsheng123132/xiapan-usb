//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestPicoProcessControlHelper(t *testing.T) {
	mode := os.Getenv("XIAPAN_PROCESS_CONTROL_TEST")
	if mode == "" {
		return
	}
	if mode == "child" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if mode == "parent" {
		child := exec.Command(os.Args[0], "-test.run=^TestPicoProcessControlHelper$")
		child.Env = append(os.Environ(), "XIAPAN_PROCESS_CONTROL_TEST=child")
		quietProcess(child)
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("XIAPAN_PROCESS_CONTROL_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			_ = child.Process.Kill()
			os.Exit(3)
		}
		_ = child.Process.Release()
	}
	os.Exit(0)
}

func picoControlTestCommand(mode, pidFile string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPicoProcessControlHelper$")
	cmd.Env = append(os.Environ(), "XIAPAN_PROCESS_CONTROL_TEST="+mode, "XIAPAN_PROCESS_CONTROL_PID_FILE="+pidFile)
	return cmd
}

func TestPicoProcessControlKeepsIdentityAndCleansTree(t *testing.T) {
	for _, action := range []string{"kill", "close"} {
		t.Run(action, func(t *testing.T) {
			// This fixture remains outside the controlled job throughout the test.
			other := picoControlTestCommand("child", "")
			quietProcess(other)
			if err := other.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })
			otherHandle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(other.Process.Pid))
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.CloseHandle(otherHandle)

			pidFile := filepath.Join(t.TempDir(), "child.pid")
			cmd := picoControlTestCommand("parent", pidFile)
			preparePicoProcess(cmd)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			control, err := newPicoProcessControl(cmd)
			if err != nil {
				_ = cmd.Wait()
				t.Fatal(err)
			}
			defer control.close()
			if err = cmd.Wait(); err != nil {
				t.Fatal(err)
			}
			if control.alive() {
				t.Fatal("exited leader reported alive after cmd.Wait")
			}
			data, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			child, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.CloseHandle(child)
			if status, err := syscall.WaitForSingleObject(child, 0); err != nil || status != syscall.WAIT_TIMEOUT {
				t.Fatalf("child should survive leader exit before cleanup: %d %v", status, err)
			}
			// A stale/reused PID cannot retarget kill: cleanup uses held handles.
			cmd.Process.Pid = other.Process.Pid
			if action == "kill" {
				control.kill()
			} else {
				control.close()
			}
			if status, err := syscall.WaitForSingleObject(child, 5000); err != nil || status != syscall.WAIT_OBJECT_0 {
				t.Fatalf("owned child survived cleanup: %d %v", status, err)
			}
			if status, err := syscall.WaitForSingleObject(otherHandle, 0); err != nil || status != syscall.WAIT_TIMEOUT {
				t.Fatalf("unrelated process was affected: %d %v", status, err)
			}
			control.close()
			control.close()
			control.kill()
			if control.alive() || control.process != 0 || control.job != 0 {
				t.Fatal("closed controller retained handles or remained alive")
			}
		})
	}
}

func TestPicoProcessControlRejectsUnpreparedAndKillsOwnedProcess(t *testing.T) {
	cmd := picoControlTestCommand("child", "")
	quietProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	control, err := newPicoProcessControl(cmd)
	if err == nil || control != nil {
		t.Fatalf("unprepared process accepted: %v %v", control, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("failed initialization left owned process running")
	}
}

func TestPicoProcessControlReleasesHandles(t *testing.T) {
	for i := 0; i < 3; i++ {
		cmd := picoControlTestCommand("child", "")
		preparePicoProcess(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		control, err := newPicoProcessControl(cmd)
		if err != nil {
			_ = cmd.Wait()
			t.Fatal(err)
		}
		if !control.alive() {
			t.Fatal("running process reported dead")
		}
		control.kill()
		_ = cmd.Wait()
		ownedHandles := []syscall.Handle{control.process, control.job}
		control.close()
		for _, handle := range ownedHandles {
			var flags uint32
			ok, _, err := picoKernel32.NewProc("GetHandleInformation").Call(uintptr(handle), uintptr(unsafe.Pointer(&flags)))
			if ok != 0 || err != syscall.Errno(6) { // ERROR_INVALID_HANDLE
				t.Fatalf("owned handle remained valid after close: handle=%v result=%v error=%v", handle, ok, err)
			}
		}
	}
}
