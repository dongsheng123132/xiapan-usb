package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"

	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// gatewayPIDLocked also recognizes replacements started by our own launcher.
// The PID file is a hint, never authority to terminate or adopt another process.
// Demand-driven actions use this check, then verify the endpoint; status never reads this file.
func (g *picoGatewayRuntime) gatewayPIDLocked() int {
	if picoAlive(g.gateway) {
		return g.gateway.cmd.Process.Pid
	}
	if g.stateLock == nil || !picoAlive(g.launcher) {
		return 0
	}
	path, err := writablePath(g.root, "data", "pico", ".picoclaw.pid")
	if err != nil {
		return 0
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return 0
	}
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return 0
	}
	var record struct {
		PID  int    `json:"pid"`
		Port int    `json:"port"`
		Host string `json:"host"`
	}
	if json.Unmarshal(data, &record) != nil || record.PID <= 0 || record.Host != "127.0.0.1" || net.JoinHostPort(record.Host, strconv.Itoa(record.Port)) != g.address {
		return 0
	}
	if record.PID == g.launcher.cmd.Process.Pid || !picoLauncherOwnsProcess(record.PID, g.launcher.cmd.Process.Pid) {
		return 0
	}
	return record.PID
}

// A native Restart briefly leaves the original exec.Cmd dead. Keep the panel,
// MCP listener and state lock alive while the launcher starts its replacement.
// Failure never kills the panel or starts a competing writer to the same state.
func (g *picoGatewayRuntime) waitLauncherGatewayLocked(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !picoAlive(g.launcher) {
			return errors.New("官方 PicoClaw 界面已退出，请重试启动 AI 服务")
		}
		pid := g.gatewayPIDLocked()
		if pid > 0 {
			if pid == g.verifiedGatewayPID {
				return nil
			}
			if g.verifyReplacementGateway(ctx, pid) {
				g.verifiedGatewayPID = pid
				g.statusSnapshotLocked()
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			return errors.New("官方 PicoClaw 服务正在重启或已停止，请在官方界面启动服务后重试")
		case <-ticker.C:
		}
	}
}

func (g *picoGatewayRuntime) verifyReplacementGateway(ctx context.Context, pid int) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+g.address+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := localHTTP(500 * time.Millisecond).Do(req)
	if err != nil {
		return false
	}
	var health struct {
		PID int `json:"pid"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&health)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || health.PID != pid {
		return false
	}
	// PID-file token changes on every launch; authentication uses the separate,
	// unchanged Pico channel token. This handshake does not submit model work.
	dialer := websocket.Dialer{HandshakeTimeout: time.Second, Subprotocols: []string{"token." + g.token}, Proxy: nil}
	conn, _, err := dialer.DialContext(ctx, "ws://"+g.address+"/pico/ws?session_id=xiapan-handoff-"+uniqueID(), nil)
	if err != nil {
		return false
	}
	conn.Close()
	return picoAlive(g.launcher) && g.gatewayPIDLocked() == pid
}

// Inspection never touches USB files or makes network requests. During a native
// restart, status stays transitional until a demand-driven handshake verifies
// the replacement. Cached snapshots keep inspection responsive during startup.
type picoStatusSnapshot struct {
	running, uiRunning bool
	url                string
}

func (g *picoGatewayRuntime) statusSnapshotLocked() *picoStatusSnapshot {
	snapshot := &picoStatusSnapshot{running: picoAlive(g.gateway), uiRunning: picoAlive(g.launcher)}
	if snapshot.uiRunning {
		snapshot.url = g.launcherURL
		if !snapshot.running && g.verifiedGatewayPID > 0 {
			snapshot.running = picoLauncherOwnsProcess(g.verifiedGatewayPID, g.launcher.cmd.Process.Pid)
		}
	}
	g.lastStatus.Store(snapshot)
	return snapshot
}
