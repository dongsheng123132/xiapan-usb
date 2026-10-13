//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	picoKernel32     = syscall.NewLazyDLL("kernel32.dll")
	picoCreateJob    = picoKernel32.NewProc("CreateJobObjectW")
	picoSetJobInfo   = picoKernel32.NewProc("SetInformationJobObject")
	picoAssignJob    = picoKernel32.NewProc("AssignProcessToJobObject")
	picoTerminateJob = picoKernel32.NewProc("TerminateJobObject")
	picoQueryJob     = picoKernel32.NewProc("QueryInformationJobObject")
	picoThreadFirst  = picoKernel32.NewProc("Thread32First")
	picoThreadNext   = picoKernel32.NewProc("Thread32Next")
	picoOpenThread   = picoKernel32.NewProc("OpenThread")
	picoResumeThread = picoKernel32.NewProc("ResumeThread")
)

const picoCreateSuspended = 0x00000004

type picoJobBasicLimits struct {
	ProcessUserTime, JobUserTime int64
	Flags                        uint32
	MinimumWorkingSet            uintptr
	MaximumWorkingSet            uintptr
	ActiveProcesses              uint32
	Affinity                     uintptr
	PriorityClass                uint32
	SchedulingClass              uint32
}

type picoJobExtendedLimits struct {
	Basic picoJobBasicLimits
	// Win32 LARGE_INTEGER structures have eight-byte alignment even on x86.
	_                 [8 - unsafe.Sizeof(uintptr(0))]byte
	IOCounters        [6]uint64
	ProcessMemory     uintptr
	JobMemory         uintptr
	PeakProcessMemory uintptr
	PeakJobMemory     uintptr
}

// These handles belong to this invocation and survive exec.Cmd.Wait. A PID
// from a status file, or one reused after Wait, never authorizes termination.
type picoProcessControl struct {
	mu      sync.Mutex
	process syscall.Handle
	job     syscall.Handle
}

func preparePicoProcess(cmd *exec.Cmd) {
	quietProcess(cmd)
	cmd.SysProcAttr.CreationFlags |= picoCreateSuspended
}

// Call immediately after Start, before any concurrent Wait. Suspending the
// initial thread keeps launcher children inside the job from their creation.
func newPicoProcessControl(cmd *exec.Cmd) (control *picoProcessControl, err error) {
	if cmd == nil || cmd.Process == nil {
		return nil, errors.New("PicoClaw 进程尚未启动")
	}
	control = &picoProcessControl{}
	defer func() {
		if err != nil {
			// Kill uses the original os.Process handle, never a numeric PID lookup.
			_ = cmd.Process.Kill()
			control.close()
			control = nil
		}
	}()
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&picoCreateSuspended == 0 {
		return control, errors.New("PicoClaw 进程未以受控方式启动")
	}
	const processAccess = syscall.PROCESS_QUERY_INFORMATION | syscall.PROCESS_TERMINATE | 0x0100 | syscall.SYNCHRONIZE // PROCESS_SET_QUOTA
	control.process, err = syscall.OpenProcess(processAccess, false, uint32(cmd.Process.Pid))
	if err != nil {
		return control, fmt.Errorf("持有 PicoClaw 进程句柄失败：%w", err)
	}
	job, _, callErr := picoCreateJob.Call(0, 0)
	if job == 0 {
		return control, fmt.Errorf("创建 PicoClaw 进程任务失败：%w", callErr)
	}
	control.job = syscall.Handle(job)
	var limits picoJobExtendedLimits
	limits.Basic.Flags = 0x00002000 // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if ok, _, callErr := picoSetJobInfo.Call(job, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits)); ok == 0 {
		return control, fmt.Errorf("设置 PicoClaw 进程任务失败：%w", callErr)
	}
	if ok, _, callErr := picoAssignJob.Call(job, uintptr(control.process)); ok == 0 {
		return control, fmt.Errorf("管理 PicoClaw 进程任务失败：%w", callErr)
	}
	if err = resumePicoInitialThread(uint32(cmd.Process.Pid)); err != nil {
		return control, err
	}
	return control, nil
}

func resumePicoInitialThread(pid uint32) error {
	const snapshotThreads = 0x00000004
	snapshot, err := syscall.CreateToolhelp32Snapshot(snapshotThreads, 0)
	if err != nil {
		return fmt.Errorf("读取 PicoClaw 初始线程失败：%w", err)
	}
	defer syscall.CloseHandle(snapshot)
	var entry struct {
		Size, Usage, ID, OwnerPID uint32
		BasePriority, Delta       int32
		Flags                     uint32
	}
	entry.Size = uint32(unsafe.Sizeof(entry))
	for ok, _, _ := picoThreadFirst.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&entry))); ok != 0; ok, _, _ = picoThreadNext.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&entry))) {
		if entry.OwnerPID != pid {
			continue
		}
		// Before resumption our freshly created process has only its initial
		// thread. The process handle pins its identity throughout this lookup.
		thread, _, callErr := picoOpenThread.Call(0x0002, 0, uintptr(entry.ID)) // THREAD_SUSPEND_RESUME
		if thread == 0 {
			return fmt.Errorf("打开 PicoClaw 初始线程失败：%w", callErr)
		}
		previous, _, callErr := picoResumeThread.Call(thread)
		_ = syscall.CloseHandle(syscall.Handle(thread))
		if uint32(previous) == ^uint32(0) {
			return fmt.Errorf("恢复 PicoClaw 初始线程失败：%w", callErr)
		}
		if previous != 1 {
			return errors.New("PicoClaw 初始线程挂起状态异常")
		}
		return nil
	}
	return errors.New("未找到 PicoClaw 初始线程")
}

func (p *picoProcessControl) alive() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.process == 0 {
		return false
	}
	status, err := syscall.WaitForSingleObject(p.process, 0)
	return err == nil && status == syscall.WAIT_TIMEOUT
}

func (p *picoProcessControl) kill() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job != 0 {
		_, _, _ = picoTerminateJob.Call(uintptr(p.job), 1)
	}
	if p.process != 0 {
		_ = syscall.TerminateProcess(p.process, 1)
	}
	// TerminateJobObject is asynchronous. Wait briefly for every owned child,
	// including a replacement gateway whose launcher has already exited.
	if p.job != 0 {
		deadline := time.Now().Add(2 * time.Second)
		for {
			var accounting struct {
				UserTime, KernelTime, PeriodUserTime, PeriodKernelTime  int64
				PageFaults, TotalProcesses, ActiveProcesses, Terminated uint32
			}
			ok, _, _ := picoQueryJob.Call(uintptr(p.job), 1, uintptr(unsafe.Pointer(&accounting)), unsafe.Sizeof(accounting), 0)
			if ok == 0 || accounting.ActiveProcesses == 0 || !time.Now().Before(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// Closing the last non-inherited job handle also cleans up any launcher child
// still alive after its parent exits. This remains safe after cmd.Wait.
func (p *picoProcessControl) close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job != 0 {
		_ = syscall.CloseHandle(p.job)
		p.job = 0
	}
	if p.process != 0 {
		_ = syscall.CloseHandle(p.process)
		p.process = 0
	}
}

func picoLauncherOwnsProcess(pid, launcherPID int) bool {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(snapshot)
	entry := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		if int(entry.ProcessID) == pid {
			return int(entry.ParentProcessID) == launcherPID && picoProcessExists(pid)
		}
	}
	return false
}
func picoProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	return syscall.GetExitCodeProcess(handle, &code) == nil && code == 259 // STILL_ACTIVE
}
