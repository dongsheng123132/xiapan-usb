package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPicoFileLockAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.lock")
	lock, err := acquirePicoFileLock(path)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestPicoFileLockChild$")
	child.Env = append(os.Environ(), "XIAPAN_LOCK_FIXTURE="+path)
	output, err := child.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "LOCKED") {
		lock.Close()
		t.Fatalf("second process acquired live lock: %v %s", err, output)
	}
	lock.Close()
	child = exec.Command(os.Args[0], "-test.run=^TestPicoFileLockChild$")
	child.Env = append(os.Environ(), "XIAPAN_LOCK_FIXTURE="+path)
	output, err = child.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "ACQUIRED") {
		t.Fatalf("closed lock did not release: %v %s", err, output)
	}
}
func TestPicoFileLockChild(t *testing.T) {
	path := os.Getenv("XIAPAN_LOCK_FIXTURE")
	if path == "" {
		t.Skip("child only")
	}
	lock, err := acquirePicoFileLock(path)
	if err != nil {
		os.Stdout.WriteString("LOCKED\n")
		return
	}
	defer lock.Close()
	os.Stdout.WriteString("ACQUIRED\n")
}
