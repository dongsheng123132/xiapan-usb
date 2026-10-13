package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPicoHandoffRejectsUnownedPIDAndEndpoint(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "data", "pico")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.Create(filepath.Join(home, ".test-lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := os.MkdirAll(filepath.Join(home, "workspace"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XIAPAN_TEST_HANDOFF_CHILD", "1")
	panel, err := startPicoOwned(root, os.Args[0], os.Args[0], []string{"-test.run=^TestPicoHandoffProcessHelper$"})
	if err != nil {
		t.Fatal(err)
	}
	defer stopPicoProcess(panel)
	g := &picoGatewayRuntime{root: root, address: "127.0.0.1:12345", launcher: panel, stateLock: lock, token: "fixture-token"}
	for _, pid := range []int{os.Getpid(), os.Getppid()} {
		b, _ := json.Marshal(map[string]any{"pid": pid, "host": "127.0.0.1", "port": 12345})
		if err := os.WriteFile(filepath.Join(home, ".picoclaw.pid"), b, 0600); err != nil {
			t.Fatal(err)
		}
		if got := g.gatewayPIDLocked(); got != 0 {
			t.Fatalf("adopted unowned PID %d", got)
		}
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"pid":1234}`))
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	g.address = strings.TrimPrefix(server.URL, "http://")
	if g.verifyReplacementGateway(context.Background(), 4321) || calls != 1 {
		t.Fatal("mismatched health PID reached handshake")
	}
	if g.verifyReplacementGateway(context.Background(), 1234) || calls != 3 {
		t.Fatal("unauthenticated endpoint adopted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.waitLauncherGatewayLocked(ctx); err != context.Canceled {
		t.Fatalf("cancellation not propagated: %v", err)
	}
	if !picoAlive(panel) || g.launcher != panel {
		t.Fatal("failed adoption closed the panel")
	}
	t.Log("rejected unrelated PID, mismatched health PID and unauthorized WebSocket; cancellation preserves panel PID " + strconv.Itoa(panel.cmd.Process.Pid))
}

func TestPicoHandoffProcessHelper(t *testing.T) {
	if os.Getenv("XIAPAN_TEST_HANDOFF_CHILD") != "1" {
		return
	}
	time.Sleep(time.Minute)
}

func TestPicoInspectDoesNotWaitForStartup(t *testing.T) {
	root := t.TempDir()
	g := picoRuntime(root)
	g.lastStatus.Store(&picoStatusSnapshot{running: true, uiRunning: true, url: "http://127.0.0.1:12345"})
	g.mu.Lock()
	defer g.mu.Unlock()
	completed := make(chan any, 1)
	go func() {
		state, err := picoGatewayInspect(root)
		if err != nil {
			completed <- err
		} else {
			completed <- state
		}
	}()
	select {
	case result := <-completed:
		state, ok := result.(map[string]any)
		if !ok || state["busy"] != true || state["running"] != true || state["url"] != "http://127.0.0.1:12345" {
			t.Fatalf("lost snapshot during startup: %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("read-only status waited behind startup lock")
	}
}
