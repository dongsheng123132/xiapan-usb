package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type picoManagedProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
}
type picoGatewayRuntime struct {
	mu            sync.Mutex
	root          string
	launcherPort  int
	engine        string
	address       string
	token         string
	gateway       *picoManagedProcess
	launcher      *picoManagedProcess
	launcherURL   string
	mcpStop       func()
	cancel        context.CancelFunc
	stateLock     *os.File
	restartNeeded atomic.Bool
	activeTurns   atomic.Int32
	bindMu        sync.Mutex
	bindings      map[string]*ChatSession
}

var picoRuntimes sync.Map

func picoRuntime(root string) *picoGatewayRuntime {
	root, _ = filepath.Abs(root)
	value, _ := picoRuntimes.LoadOrStore(root, &picoGatewayRuntime{root: root, bindings: map[string]*ChatSession{}})
	return value.(*picoGatewayRuntime)
}
func picoAlive(p *picoManagedProcess) bool {
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func picoGatewayInspect(root string) (any, error) {
	p := picoManifest()
	slot, available := p.currentSlot()
	data := map[string]any{"version": p.Version, "distribution": p.Distribution, "available": available, "running": false, "ui_running": false, "portable": true, "updates_enabled": false, "ui_available": slot.LauncherPath != ""}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if value, ok := picoRuntimes.Load(absolute); ok {
		g := value.(*picoGatewayRuntime)
		g.mu.Lock()
		defer g.mu.Unlock()
		data["running"] = picoAlive(g.gateway)
		data["ui_running"] = picoAlive(g.launcher)
		if picoAlive(g.launcher) {
			data["url"] = g.launcherURL
		}
	}
	return data, nil
}
func picoGatewayStart(ctx context.Context, root string) (any, error) {
	if _, err := ensurePicoGateway(ctx, root); err != nil {
		return nil, err
	}
	return picoGatewayInspect(root)
}
func picoGatewayStop(root string) (any, error) {
	stopOwnedPicoProcesses(root)
	return picoGatewayInspect(root)
}

// Called by the action core after wallet/settings locks have been released.
// Closing the old process before returning prevents the advanced UI spending
// against an obsolete key. No disk PID is used as authority to terminate.
func syncOwnedPicoCredentials(ctx context.Context, root string) error {
	absolute, _ := filepath.Abs(root)
	value, ok := picoRuntimes.Load(absolute)
	if !ok {
		return nil
	}
	g := value.(*picoGatewayRuntime)
	if !g.restartNeeded.Load() {
		return nil
	}
	g.mu.Lock()
	wasRunning := picoAlive(g.gateway)
	hadUI := picoAlive(g.launcher)
	if !wasRunning && !hadUI {
		g.mu.Unlock()
		return nil
	}
	g.stopLocked()
	g.mu.Unlock()
	selection, err := selectedModel(root)
	if err != nil {
		return err
	}
	key := selection.APIKey
	if selection.Source == "cloud" {
		key, err = walletKey(root)
		if err != nil {
			return nil
		}
	}
	if key == "" {
		return nil
	}
	if _, err = ensurePicoGateway(ctx, root); err != nil {
		return err
	}
	if hadUI {
		_, err = picoWebUIStart(ctx, root)
	}
	return err
}

func stopAllOwnedPicoProcesses() {
	picoRuntimes.Range(func(_, v any) bool { g := v.(*picoGatewayRuntime); stopOwnedPicoProcesses(g.root); return true })
}
func stopOwnedPicoProcesses(root string) {
	absolute, _ := filepath.Abs(root)
	v, ok := picoRuntimes.Load(absolute)
	if !ok {
		return
	}
	g := v.(*picoGatewayRuntime)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopLocked()
}
func stopPicoProcess(p *picoManagedProcess) {
	if !picoAlive(p) {
		return
	}
	// A PID from disk is never a kill target; only our exec.Cmd is owned.
	_ = p.cmd.Process.Signal(os.Interrupt)
	select {
	case <-p.done:
		return
	case <-time.After(1500 * time.Millisecond):
	}
	killPicoProcessTree(p.cmd)
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
	}
}
func (g *picoGatewayRuntime) stopLocked() {
	stopPicoProcess(g.launcher)
	g.launcher = nil
	g.launcherURL = ""
	stopPicoProcess(g.gateway)
	g.gateway = nil
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	if g.mcpStop != nil {
		g.mcpStop()
		g.mcpStop = nil
	}
	if g.stateLock != nil {
		g.stateLock.Close()
		g.stateLock = nil
	}
}
func freePicoPort() (int, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
func newPicoToken() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func ensurePicoGateway(ctx context.Context, root string) (*picoGatewayRuntime, error) {
	g := picoRuntime(root)
	g.mu.Lock()
	defer g.mu.Unlock()
	if picoAlive(g.gateway) && !g.restartNeeded.Load() {
		return g, nil
	}
	if picoAlive(g.gateway) && g.activeTurns.Load() > 0 {
		return nil, errors.New("模型设置已保存；请等待当前任务完成后重试，新任务会使用新凭证")
	}
	g.stopLocked()
	lockPath, err := writablePath(root, "data", "pico", ".xiapan-gateway.lock")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	stateLock, err := acquirePicoFileLock(lockPath)
	if err != nil {
		return nil, errors.New("此 U 盘的 AI 服务已由另一个窗口使用，请返回原窗口或先退出它")
	}
	g.stateLock = stateLock
	keepLock := false
	defer func() {
		if !keepLock {
			g.stopLocked()
		}
	}()
	// After a crashed wrapper a native child may still own the same data. Never
	// overwrite its config, and never use an on-disk PID as a kill target.
	pidPath, _ := writablePath(root, "data", "pico", ".picoclaw.pid")
	if b, e := os.ReadFile(pidPath); e == nil {
		var previous struct {
			Port int `json:"port"`
		}
		if json.Unmarshal(b, &previous) == nil && previous.Port > 0 && previous.Port < 65536 {
			if resp, e := localHTTP(400 * time.Millisecond).Get("http://127.0.0.1:" + strconv.Itoa(previous.Port) + "/health"); e == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil, errors.New("本盘已有 PicoClaw 服务运行，请先从原窗口退出；未改写配置或结束其他进程")
				}
			}
		}
	}
	p := picoManifest()
	slot, ok := p.currentSlot()
	if !ok {
		return nil, errors.New("此系统尚未准备 PicoClaw 运行时")
	}
	engine, err := cacheRuntimeResource(root, slot.Path, slot.SHA)
	if err != nil {
		return nil, err
	}
	selection, err := selectedModel(root)
	if err != nil {
		return nil, err
	}
	key := selection.APIKey
	if selection.Source == "cloud" {
		key, err = walletKey(root)
		if err != nil {
			return nil, err
		}
	}
	if key == "" {
		return nil, errors.New("请先启用设备钱包或填写自带 API Key")
	}
	if err = applyPicoCredential(root, key); err != nil {
		return nil, err
	}
	port, err := freePicoPort()
	if err != nil {
		return nil, err
	}
	token, err := newPicoToken()
	if err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(context.Background())
	mcp, stop, err := startGatewayPicoMCP(lifetime, root, g)
	if err != nil {
		cancel()
		return nil, err
	}
	g.cancel = cancel
	g.mcpStop = stop
	lock := rootLock(&picoConfigLocks, root)
	lock.Lock()
	cfg, err := readPicoMap(root, "config.json", false)
	if err == nil {
		gateway := picoMap(cfg, "gateway")
		gateway["host"] = "127.0.0.1"
		gateway["port"] = port
		gateway["hot_reload"] = true
		channel := picoMap(picoMap(cfg, "channel_list"), "pico")
		channel["type"] = "pico"
		channel["enabled"] = true
		channel["typing"] = map[string]any{"enabled": true}
		channel["placeholder"] = map[string]any{"enabled": false}
		settings := picoMap(channel, "settings")
		settings["streaming"] = map[string]any{"enabled": true}
		settings["read_timeout"] = 1800
		delete(settings, "token")
		picoMap(picoMap(picoMap(cfg, "tools"), "mcp"), "servers")["xiapan"] = mcp
		picoMap(picoMap(cfg, "tools"), "mcp")["enabled"] = true
		security, e := readPicoMap(root, ".security.yml", true)
		err = e
		if err == nil {
			picoMap(picoMap(picoMap(security, "channel_list"), "pico"), "settings")["token"] = token
			err = writePicoMap(root, ".security.yml", security, true)
		}
		if err == nil {
			err = writePicoMap(root, "config.json", cfg, false)
		}
	}
	lock.Unlock()
	if err != nil {
		g.stopLocked()
		return nil, err
	}
	g.address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	g.token = token
	g.engine = engine
	process, err := startPicoOwned(root, engine, engine, []string{"gateway", "-E", "--no-color"})
	if err != nil {
		g.stopLocked()
		return nil, err
	}
	g.gateway = process
	if err = waitPicoHTTP(ctx, "http://"+g.address+"/health", process); err != nil {
		g.stopLocked()
		return nil, err
	}
	g.restartNeeded.Store(false)
	keepLock = true
	return g, nil
}
func startPicoOwned(root, exe, engine string, args []string) (*picoManagedProcess, error) {
	env, err := picoEnvironment(root, engine)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	preparePicoProcess(cmd)
	cmd.Env = env
	home, err := writablePath(root, "data", "pico")
	if err != nil {
		return nil, err
	}
	cmd.Dir = filepath.Join(home, "workspace")
	// Native launcher stores its own logs. Avoid another high-volume USB log.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if os.Getenv("XIAPAN_PICO_DEBUG") == "1" {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stderr
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("PicoClaw 启动失败：%w", err)
	}
	p := &picoManagedProcess{cmd: cmd, done: make(chan struct{})}
	go func() { cmd.Wait(); close(p.done) }()
	return p, nil
}
func waitPicoHTTP(ctx context.Context, address string, p *picoManagedProcess) error {
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	client := localHTTP(500 * time.Millisecond)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if resp, e := client.Do(req); e == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.done:
			return errors.New("PicoClaw 已退出，请检查随盘模型配置")
		case <-deadline.C:
			return errors.New("PicoClaw 启动未就绪，请重试；离线工具仍可使用")
		case <-tick.C:
		}
	}
}
func picoWebUIStart(ctx context.Context, root string) (any, error) {
	g, err := ensurePicoGateway(ctx, root)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !picoAlive(g.launcher) {
		slot, _ := picoManifest().currentSlot()
		if slot.LauncherPath == "" {
			return nil, errors.New("本盘未准备官方 PicoClaw 界面")
		}
		launcher, e := cacheRuntimeResource(root, slot.LauncherPath, slot.LauncherSHA)
		if e != nil {
			return nil, e
		}
		port := g.launcherPort
		if port == 0 {
			port, e = freePicoPort()
			if e != nil {
				return nil, e
			}
		}
		g.launcherPort = port
		p, e := startPicoOwned(root, launcher, g.engine, []string{"-console", "-no-browser", "-host", "127.0.0.1", "-port", strconv.Itoa(port), filepath.Join(root, "data", "pico", "config.json")})
		if e != nil {
			return nil, e
		}
		g.launcher = p
		g.launcherURL = "http://127.0.0.1:" + strconv.Itoa(port)
		if e = waitPicoHTTP(ctx, g.launcherURL+"/", p); e != nil {
			stopPicoProcess(p)
			g.launcher = nil
			return nil, e
		}
	}
	return map[string]any{"url": g.launcherURL, "opened": false, "note": "首次进入官方界面请设置本地面板密码；API Key、技能和原生会话保存在本盘。"}, nil
}

type picoWireMessage struct {
	Type      string         `json:"type"`
	SessionID string         `json:"session_id"`
	Payload   map[string]any `json:"payload"`
}

func picoGatewayChat(ctx context.Context, g *picoGatewayRuntime, sessionID, prompt string) (string, error) {
	g.mu.Lock()
	address, token := g.address, g.token
	g.mu.Unlock()
	dialer := websocket.Dialer{HandshakeTimeout: 8 * time.Second, Subprotocols: []string{"token." + token}, Proxy: nil}
	conn, _, err := dialer.DialContext(ctx, "ws://"+address+"/pico/ws?session_id="+url.QueryEscape(sessionID), nil)
	if err != nil {
		return "", errors.New("PicoClaw 会话连接失败，请重启 AI 服务")
	}
	defer conn.Close()
	conn.SetReadLimit(8 * 1024 * 1024)
	if err = conn.WriteJSON(map[string]any{"type": "message.send", "id": uniqueID(), "payload": map[string]any{"content": prompt}}); err != nil {
		return "", err
	}
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			conn.SetWriteDeadline(time.Now().Add(time.Second))
			_ = conn.WriteJSON(map[string]any{"type": "message.send", "id": uniqueID(), "payload": map[string]any{"content": "/stop"}})
			conn.Close()
		case <-finished:
		}
	}()
	messages := map[string]string{}
	order := []string{}
	kinds := map[string]string{}
	placeholders := map[string]bool{}
	typingStopped := false
	answer := func() string {
		texts := []string{}
		for _, id := range order {
			if kinds[id] != "thought" && kinds[id] != "tool_calls" && !placeholders[id] && strings.TrimSpace(messages[id]) != "" {
				texts = append(texts, messages[id])
			}
		}
		return strings.Join(texts, "\n\n")
	}
	for {
		var msg picoWireMessage
		if err = conn.ReadJSON(&msg); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", errors.New("PicoClaw 连接中断；会话保留在本盘，可在官方界面继续")
		}
		if msg.SessionID != "" && msg.SessionID != sessionID {
			continue
		}
		p := msg.Payload
		id, _ := p["message_id"].(string)
		switch msg.Type {
		case "message.create", "message.update":
			if id == "" {
				continue
			}
			if _, exists := messages[id]; !exists {
				order = append(order, id)
			}
			if kind, ok := p["kind"].(string); ok {
				kinds[id] = kind
			}
			if p["thought"] == true {
				kinds[id] = "thought"
			}
			if placeholder, ok := p["placeholder"].(bool); ok {
				placeholders[id] = placeholder
			} else if msg.Type == "message.update" {
				placeholders[id] = false
			}
			if content, ok := p["content"].(string); ok {
				messages[id] = content
			}
			if typingStopped && kinds[id] != "thought" && kinds[id] != "tool_calls" && !placeholders[id] {
				if result := answer(); result != "" {
					return result, nil
				}
			}
		case "message.delete":
			delete(messages, id)
		case "typing.start":
			typingStopped = false
		case "typing.stop":
			// v0.3.1 preSend emits typing.stop BEFORE a non-streamed final message.
			typingStopped = true
			if result := answer(); result != "" {
				return result, nil
			}

		case "error":
			return "", errors.New("PicoClaw 未完成本轮任务，请在官方界面查看服务状态与日志")
		}
	}
}
