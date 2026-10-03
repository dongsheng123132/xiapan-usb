package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The engine is Open365's network.ps1 (Apache-2.0), vendored unchanged apart
// from one provenance comment line. Only its read-only `diagnose` action is
// ever run. See THIRD_PARTY.md.
//
//go:embed engine/network.ps1
var networkEngineScript []byte

const (
	networkUnsupported   = "断网急救目前仅支持 Windows"
	networkEngineTimeout = 90 * time.Second
	toolNetworkSettings  = "network-settings"
	toolDeviceManager    = "device-manager"
)

// Test seams. Production code never replaces them; tests do, so that no test
// runs PowerShell or depends on the host operating system.
var (
	networkSupported = func() bool { return runtime.GOOS == "windows" }
	runNetworkEngine = runNetworkEnginePowerShell
)

// ensureNetworkScript writes the embedded engine to data/tmp under a name that
// carries the first 12 hex digits of its SHA-256. A file whose hash already
// matches is reused; anything else under that name is replaced.
func ensureNetworkScript(root string, content []byte) (string, error) {
	sum := sha256.Sum256(content)
	path, err := writablePath(root, "data", "tmp", "network-engine-"+hex.EncodeToString(sum[:])[:12]+".ps1")
	if err != nil {
		return "", err
	}
	if existing, err := os.ReadFile(path); err == nil && sha256.Sum256(existing) == sum {
		return path, nil
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".xiapan-script-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

func parseEngineJSON(b []byte) (map[string]any, error) {
	b = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(b), []byte("\xef\xbb\xbf")))
	var m map[string]any
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&m); err != nil || m == nil {
		return nil, errors.New("网络引擎的输出无法解析")
	}
	return m, nil
}

// runNetworkEnginePowerShell runs the engine's read-only diagnosis. The command
// line is fixed; nothing from user input reaches it.
func runNetworkEnginePowerShell(ctx context.Context, root string) (map[string]any, error) {
	dir := nativeSystemDir()
	if dir == "" {
		return nil, errors.New("无法定位 Windows 系统目录")
	}
	script, err := ensureNetworkScript(root, networkEngineScript)
	if err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, networkEngineTimeout)
	defer cancel()
	cmd := exec.CommandContext(deadline, filepath.Join(dir, "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "diagnose", "-Json")
	quietProcess(cmd)
	if cmd.Env, err = portableEnv(root); err != nil {
		return nil, err
	}
	b, err := cmd.Output()
	if err != nil {
		if deadline.Err() != nil {
			return nil, errors.New("网络诊断超时或被取消；未改动任何设置")
		}
		detail := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			detail = strings.TrimSpace(strings.ToValidUTF8(string(exit.Stderr), ""))
			if len(detail) > 300 {
				detail = detail[:300]
			}
		}
		return nil, fmt.Errorf("网络诊断未能完成：%v %s", err, detail)
	}
	return parseEngineJSON(b)
}

// ---- next step: a pure function of the engine's measurements ----

type NetworkTests struct {
	Gateway, Internet, DNS, WebDirect, WebProxy bool
}
type NetworkProxy struct {
	Enabled bool
	Server  string
}

// NextStep names one registered system tool to open and says what to do in it.
// Opening still goes through tools.launch and its confirmation.
type NextStep struct{ ToolID, Reason string }

// The reasons are fixed strings so that the interface can translate them.
const (
	reasonProxy      = "系统代理指向外部地址，网页因此打不开：在打开的设置里进入「代理」，关闭手动代理。"
	reasonLocalProxy = "本机代理（如 Clash/V2Ray）无法上网：先确认该软件在运行；若不再使用，在打开的设置里进入「代理」关闭它。"
	reasonNoAdapter  = "没有检测到已连接的网卡：先确认网线已插好或 Wi-Fi 已打开，再在设备管理器的「网络适配器」里看是否缺失、带黄色感叹号或被禁用；缺驱动时按硬件 ID 去厂商官网准备。"
	reasonDNS        = "能连上外网 IP 但域名解析失败：在打开的设置里进入当前网络的属性，把 DNS 服务器改成公共 DNS（如 223.5.5.5）；仍不行就在「网络重置」中重置，重启后生效。"
	reasonWinsock    = "IP 和 DNS 都正常但网页打不开，疑似 Winsock/TCP-IP 损坏：在打开的设置里进入「网络重置」重置网络适配器，重启后生效。"
	reasonRouter     = "能连上路由器但出不了外网：先检查路由器或光猫的网络并重启它们；仍不行，在打开的设置里查看连接状态，或用「网络重置」。"
	reasonGateway    = "连不上网关：先检查网线或 Wi-Fi 是否已连接、路由器是否通电；在打开的设置里可查看连接状态，必要时用「网络重置」。"
)

// proxyIsLoopback reports whether every entry of a WinINET ProxyServer value
// points at this machine. The value may be "host:port", "scheme://host:port" or
// "http=host:port;https=host:port". Anything unrecognised is not loopback.
func proxyIsLoopback(server string) bool {
	entries := strings.FieldsFunc(server, func(r rune) bool { return r == ';' || r == ' ' || r == '\t' })
	if len(entries) == 0 {
		return false
	}
	for _, entry := range entries {
		if at := strings.Index(entry, "="); at >= 0 {
			entry = entry[at+1:]
		}
		if at := strings.Index(entry, "://"); at >= 0 {
			entry = entry[at+3:]
		}
		if at := strings.IndexAny(entry, "/?#"); at >= 0 {
			entry = entry[:at]
		}
		host := entry
		if strings.HasPrefix(host, "[") {
			end := strings.Index(host, "]")
			if end < 0 {
				return false
			}
			host = host[1:end]
		} else if strings.Count(host, ":") == 1 {
			host = host[:strings.Index(host, ":")]
		}
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return false
		}
	}
	return true
}

// networkNextStep follows the engine's own decision order on its measured values
// and never reads its Chinese verdict text. It returns nil when the web works.
// Every step opens a system tool; this program changes no network setting.
func networkNextStep(t NetworkTests, p NetworkProxy, adapters int) *NextStep {
	proxyActive := p.Enabled && strings.TrimSpace(p.Server) != ""
	web := t.WebDirect
	if proxyActive {
		web = t.WebProxy
	}
	switch {
	case web:
		return nil
	case adapters == 0:
		return &NextStep{toolDeviceManager, reasonNoAdapter}
	case proxyActive && t.WebDirect && !t.WebProxy:
		if proxyIsLoopback(p.Server) {
			return &NextStep{toolNetworkSettings, reasonLocalProxy}
		}
		return &NextStep{toolNetworkSettings, reasonProxy}
	case t.Internet && !t.DNS:
		return &NextStep{toolNetworkSettings, reasonDNS}
	case t.Internet && t.DNS && !t.WebDirect:
		return &NextStep{toolNetworkSettings, reasonWinsock}
	case t.Gateway && !t.Internet:
		return &NextStep{toolNetworkSettings, reasonRouter}
	case !t.Gateway:
		return &NextStep{toolNetworkSettings, reasonGateway}
	}
	return nil
}

// annotateNetworkResult adds next_step (null when nothing needs doing) to the
// engine's own result. It refuses an incomplete result rather than guess.
func annotateNetworkResult(r map[string]any) error {
	tests, _ := r["tests"].(map[string]any)
	complete := true
	get := func(key string) bool {
		v, ok := tests[key].(bool)
		complete = complete && ok
		return v
	}
	t := NetworkTests{Gateway: get("gateway_reachable"), Internet: get("internet_reachable"), DNS: get("dns_works"), WebDirect: get("web_direct"), WebProxy: get("web_via_proxy")}
	adapters, listed := r["adapters"].([]any)
	if !complete || !listed {
		return errors.New("网络引擎的诊断结果不完整；未给出建议")
	}
	proxy, _ := r["proxy"].(map[string]any)
	var p NetworkProxy
	p.Enabled, _ = proxy["enabled"].(bool)
	p.Server, _ = proxy["server"].(string)
	step := networkNextStep(t, p, len(adapters))
	if step == nil {
		r["next_step"] = nil
		return nil
	}
	r["next_step"] = map[string]any{"tool_id": step.ToolID, "name": builtinToolName(step.ToolID), "reason": step.Reason}
	return nil
}

// ---- network.rescue (read only) ----

func networkRescue(ctx context.Context, root string) (any, error) {
	if !networkSupported() {
		return nil, errors.New(networkUnsupported)
	}
	r, err := runNetworkEngine(ctx, root)
	if err != nil {
		return nil, err
	}
	if err = annotateNetworkResult(r); err != nil {
		return nil, err
	}
	return r, nil
}

func networkRescueHints(english bool) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	if english {
		return "When the network is down or web pages will not open, call network.rescue first. If it returns a next_step, request tools.launch for that tool_id and tell the user what to do in the tool (its reason). This app changes no network setting itself, so never promise to fix it."
	}
	return "断网或网页打不开时先调用 network.rescue。它返回 next_step 时，用 tools.launch 申请打开其中的 tool_id，并把 reason 里该在工具里做的事告诉用户。本程序自己不改任何网络设置，不要承诺替用户修好。"
}
