//go:build !windows

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestPicoProcessControlKillsStoppedLeader(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "60")
	preparePicoProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill() // Failure must not leave our fixture stopped.
	control, err := newPicoProcessControl(cmd)
	if err != nil {
		t.Fatal(err)
	}
	p := &picoManagedProcess{cmd: cmd, done: make(chan struct{}), control: control}
	go func() { cmd.Wait(); close(p.done) }()
	stopPicoProcess(p)
	select {
	case <-p.done:
	case <-time.After(time.Second):
		t.Fatal("the owned group leader remained stopped instead of exiting")
	}
	if control.alive() {
		t.Fatal("the stopped group leader survived cleanup")
	}
}
