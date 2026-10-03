package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// useFakeEngine replaces both seams. Nothing in these tests runs PowerShell,
// whatever the operating system is. It returns the number of engine runs.
func useFakeEngine(t *testing.T, supported bool, engine func() (map[string]any, error)) *int {
	t.Helper()
	runs := 0
	oldSupported, oldEngine := networkSupported, runNetworkEngine
	networkSupported = func() bool { return supported }
	runNetworkEngine = func(context.Context, string) (map[string]any, error) {
		runs++
		return engine()
	}
	t.Cleanup(func() { networkSupported, runNetworkEngine = oldSupported, oldEngine })
	return &runs
}

// diagnoseFixture has the shape Open365's `network.ps1 diagnose -Json` emits.
func diagnoseFixture(t *testing.T, adapters int, gateway, internet, dns, webDirect, webProxy bool, proxyEnabled bool, proxyServer, verdict string) map[string]any {
	t.Helper()
	web := webDirect
	if proxyEnabled && proxyServer != "" {
		web = webProxy
	}
	list := []any{}
	for i := 0; i < adapters; i++ {
		list = append(list, map[string]any{"name": "Wi-Fi", "ipv4": "192.168.1.5", "gateway": "192.168.1.1", "dns": "192.168.1.1"})
	}
	b, _ := json.Marshal(map[string]any{
		"timestamp":   "2026-10-03T10:00:00",
		"adapters":    list,
		"gateway":     "192.168.1.1",
		"dns_servers": []any{"192.168.1.1"},
		"proxy":       map[string]any{"enabled": proxyEnabled, "server": proxyServer},
		"tests":       map[string]any{"gateway_reachable": gateway, "internet_reachable": internet, "dns_works": dns, "web_direct": webDirect, "web_via_proxy": webProxy, "web_works": web},
		"verdict":     verdict,
		"suggestion":  "none",
		"hosts":       map[string]any{"entries": 0, "suspicious": false, "hits": []any{}},
	})
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestNetworkNextStepBranches(t *testing.T) {
	step := func(tool, reason string) *NextStep { return &NextStep{tool, reason} }
	cases := []struct {
		name     string
		tests    NetworkTests
		proxy    NetworkProxy
		adapters int
		want     *NextStep
	}{
		{"web works", NetworkTests{true, true, true, true, true}, NetworkProxy{}, 1, nil},
		{"web works through a local proxy", NetworkTests{true, true, true, false, true}, NetworkProxy{true, "127.0.0.1:7890"}, 1, nil},
		{"no active adapter", NetworkTests{false, false, false, false, false}, NetworkProxy{}, 0, step(toolDeviceManager, reasonNoAdapter)},
		{"dns fault", NetworkTests{true, true, false, false, false}, NetworkProxy{}, 1, step(toolNetworkSettings, reasonDNS)},
		{"external proxy is the culprit", NetworkTests{true, true, true, true, false}, NetworkProxy{true, "10.9.8.7:3128"}, 1, step(toolNetworkSettings, reasonProxy)},
		{"local proxy is the culprit", NetworkTests{true, true, true, true, false}, NetworkProxy{true, "127.0.0.1:7890"}, 1, step(toolNetworkSettings, reasonLocalProxy)},
		{"local proxy given per scheme", NetworkTests{true, true, true, true, false}, NetworkProxy{true, "http=127.0.0.1:7890;https=127.0.0.1:7890"}, 1, step(toolNetworkSettings, reasonLocalProxy)},
		{"winsock or tcp/ip", NetworkTests{true, true, true, false, false}, NetworkProxy{}, 1, step(toolNetworkSettings, reasonWinsock)},
		{"router answers but no internet", NetworkTests{true, false, false, false, false}, NetworkProxy{}, 1, step(toolNetworkSettings, reasonRouter)},
		{"gateway unreachable", NetworkTests{false, false, false, false, false}, NetworkProxy{}, 1, step(toolNetworkSettings, reasonGateway)},
		{"proxy switched on without a server is not a proxy", NetworkTests{true, true, true, true, false}, NetworkProxy{true, "  "}, 1, nil},
		{"proxy on but direct is down too is not a proxy fault", NetworkTests{true, true, false, false, false}, NetworkProxy{true, "10.9.8.7:3128"}, 1, step(toolNetworkSettings, reasonDNS)},
	}
	registered := []string{}
	for _, tool := range builtinTools {
		registered = append(registered, tool.ID)
	}
	for _, c := range cases {
		got := networkNextStep(c.tests, c.proxy, c.adapters)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
		if got != nil && (!slices.Contains(registered, got.ToolID) || got.Reason == "") {
			t.Errorf("%s: step is not a registered system tool with a reason: %+v", c.name, got)
		}
	}
}

func TestProxyIsLoopback(t *testing.T) {
	for server, want := range map[string]bool{
		"127.0.0.1:7890":                     true,
		"localhost:8080":                     true,
		"LOCALHOST":                          true,
		"[::1]:1080":                         true,
		"::1":                                true,
		"http://127.0.0.1:7890":              true,
		"http=127.0.0.1:1;https=127.0.0.1:2": true,
		"socks=127.0.0.1:1080":               true,
		"10.0.0.2:8080":                      false,
		"proxy.corp.example:3128":            false,
		"127.0.0.1.evil.example:80":          false,
		"http=127.0.0.1:1;https=10.0.0.2:2":  false,
		"":                                   false,
		"[::1":                               false,
	} {
		if got := proxyIsLoopback(server); got != want {
			t.Errorf("proxyIsLoopback(%q) = %v, want %v", server, got, want)
		}
	}
}

func TestNextStepReadsMeasurementsNotVerdictText(t *testing.T) {
	// The verdict says "DNS fault" but every measurement is healthy.
	healthy := diagnoseFixture(t, 1, true, true, true, true, true, false, "", "IP 能通但域名解析失败 —— DNS 故障。建议切换公共 DNS。")
	if err := annotateNetworkResult(healthy); err != nil {
		t.Fatal(err)
	}
	if step, present := healthy["next_step"]; !present || step != nil {
		t.Fatalf("verdict text influenced the next step: %#v", step)
	}
	// And the reverse: a reassuring verdict cannot hide a DNS fault.
	broken := diagnoseFixture(t, 1, true, true, false, false, false, false, "", "网络正常。")
	if err := annotateNetworkResult(broken); err != nil {
		t.Fatal(err)
	}
	step := broken["next_step"].(map[string]any)
	if step["tool_id"] != toolNetworkSettings || step["reason"] != reasonDNS || step["name"] == "" {
		t.Fatalf("%#v", step)
	}
	if broken["verdict"] != "网络正常。" || broken["tests"] == nil || broken["adapters"] == nil {
		t.Fatal("engine fields were not preserved")
	}
}

func TestAnnotateRefusesIncompleteResults(t *testing.T) {
	for name, r := range map[string]map[string]any{
		"no tests":        {"adapters": []any{}},
		"missing a test":  {"adapters": []any{}, "tests": map[string]any{"gateway_reachable": true}},
		"no adapter list": {"tests": map[string]any{"gateway_reachable": true, "internet_reachable": true, "dns_works": true, "web_direct": true, "web_via_proxy": true}},
		"test not a bool": {"adapters": []any{}, "tests": map[string]any{"gateway_reachable": "yes", "internet_reachable": true, "dns_works": true, "web_direct": true, "web_via_proxy": true}},
	} {
		if err := annotateNetworkResult(r); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if _, set := r["next_step"]; set {
			t.Errorf("%s: guessed a next step", name)
		}
	}
}

func TestParseEngineJSONToleratesBOM(t *testing.T) {
	m, err := parseEngineJSON([]byte("\xef\xbb\xbf  {\"ok\":true}\r\n"))
	if err != nil || m["ok"] != true {
		t.Fatalf("%v %#v", err, m)
	}
	for _, bad := range []string{"", "not json", "[]", "null"} {
		if _, err := parseEngineJSON([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestNetworkRescueRefusesOutsideWindows(t *testing.T) {
	runs := useFakeEngine(t, false, func() (map[string]any, error) { return nil, nil })
	r := runAction(context.Background(), t.TempDir(), "network.rescue", map[string]any{})
	if r.OK || !strings.Contains(r.Error, "断网急救目前仅支持 Windows") {
		t.Fatalf("%#v", r)
	}
	if *runs != 0 {
		t.Fatal("engine ran on an unsupported platform")
	}
}

func TestNetworkRescueIsReadOnlyAndHasNoRepairAction(t *testing.T) {
	runs := useFakeEngine(t, true, func() (map[string]any, error) {
		return diagnoseFixture(t, 1, true, true, false, false, false, false, "", "dns"), nil
	})
	r := runAction(context.Background(), t.TempDir(), "network.rescue", map[string]any{"fix": "flush-dns", "confirmed": true})
	if !r.OK {
		t.Fatal(r.Error)
	}
	if *runs != 1 {
		t.Fatalf("engine ran %d times", *runs)
	}
	step := r.Data.(map[string]any)["next_step"].(map[string]any)
	if step["tool_id"] != toolNetworkSettings {
		t.Fatalf("%#v", step)
	}
	for _, a := range actions {
		if a.ID == "network.rescue" && a.Writes {
			t.Fatal("network.rescue is marked as writing")
		}
		if strings.Contains(a.ID, "repair") {
			t.Fatalf("a repair action is registered: %s", a.ID)
		}
	}
	if r := runAction(context.Background(), t.TempDir(), "network.repair", map[string]any{"fix": "flush-dns", "confirmed": true}); r.OK {
		t.Fatal("network.repair ran")
	}
	if *runs != 1 {
		t.Fatal("a repair request reached the engine")
	}
}

func TestAgentCanReadRescueButNothingElseNetworkRelated(t *testing.T) {
	useFakeEngine(t, true, func() (map[string]any, error) {
		return diagnoseFixture(t, 1, true, true, true, true, true, false, "", "网络正常。"), nil
	})
	s := &ChatSession{ID: uniqueID(), Messages: []ChatMessage{}, Pending: []Proposal{}}
	out := modelTool(context.Background(), t.TempDir(), s, "network.rescue", map[string]any{"action": "network.rescue"})
	if r, ok := out.(Result); !ok || !r.OK || len(s.Messages) != 1 || s.Messages[0].Action != "network.rescue" {
		t.Fatalf("%#v / %#v", out, s.Messages)
	}
	out = modelTool(context.Background(), t.TempDir(), s, "network.repair", map[string]any{"action": "network.repair", "fix": "flush-dns", "confirmed": true})
	if m, ok := out.(map[string]any); !ok || m["ok"] != false || len(s.Pending) != 0 {
		t.Fatalf("network.repair reached the agent: %#v", out)
	}
	schema := maintenanceToolSchema()
	props := schema["properties"].(map[string]any)
	ids := props["action"].(map[string]any)["enum"].([]string)
	if !slices.Contains(ids, "network.rescue") || slices.Contains(ids, "network.repair") {
		t.Fatalf("actions: %v", ids)
	}
	if _, has := props["fix"]; has {
		t.Fatal("schema still offers a fix property")
	}
}

func TestSlashCommandsRunNetworkRescue(t *testing.T) {
	useFakeEngine(t, true, func() (map[string]any, error) {
		return diagnoseFixture(t, 1, true, true, true, true, true, false, "", "网络正常。"), nil
	})
	for _, cmd := range []string{"/断网", "/急救"} {
		s := &ChatSession{ID: uniqueID(), Messages: []ChatMessage{}, Pending: []Proposal{}}
		if err := runSlash(context.Background(), t.TempDir(), s, cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
		if len(s.Messages) != 1 || s.Messages[0].Action != "network.rescue" {
			t.Errorf("%s: %#v", cmd, s.Messages)
		}
	}
	if !chatReadActions["network.rescue"] {
		t.Fatal("network.rescue must be a read action")
	}
}

// The suggested tools must really open on Windows, and adding the settings page
// must not change how the existing .msc tools resolve.
func TestNetworkSettingsToolIsFixedAndOpensThroughTheRegistry(t *testing.T) {
	var settings *BuiltinTool
	for i := range builtinTools {
		if builtinTools[i].ID == toolNetworkSettings {
			settings = &builtinTools[i]
		}
	}
	if settings == nil || settings.Program != "rundll32.exe" || !reflect.DeepEqual(settings.Args, []string{"url.dll,FileProtocolHandler", "ms-settings:network-status"}) {
		t.Fatalf("%+v", settings)
	}
	if runtime.GOOS != "windows" {
		return
	}
	program, args, ready := builtinCommand(*settings)
	if !ready || !strings.EqualFold(filepath.Base(program), "rundll32.exe") || !reflect.DeepEqual(args, settings.Args) {
		t.Fatalf("network-settings: %s %v %v", program, args, ready)
	}
	for _, tool := range builtinTools {
		if tool.ID == "device-manager" || tool.ID == "event-viewer" {
			_, args, ready := builtinCommand(tool)
			if !ready || len(args) != 1 || !filepath.IsAbs(args[0]) || !strings.HasSuffix(args[0], ".msc") {
				t.Fatalf("%s no longer resolves its console file: %v %v", tool.ID, args, ready)
			}
		}
	}
	for _, id := range []string{toolNetworkSettings, toolDeviceManager} {
		if requireBuiltinTool(id) != nil || !slices.Contains(maintenanceToolSchema()["properties"].(map[string]any)["tool_id"].(map[string]any)["enum"].([]string), id) {
			t.Fatalf("%s is not offered to the agent", id)
		}
		s := &ChatSession{ID: uniqueID(), Messages: []ChatMessage{}, Pending: []Proposal{}}
		out := modelTool(context.Background(), t.TempDir(), s, "tools.launch", map[string]any{"tool_id": id}).(map[string]any)
		if out["status"] != "requires_user_confirmation" || len(s.Pending) != 1 || s.Pending[0].Input["confirmed"] != nil {
			t.Fatalf("%s: %#v", id, out)
		}
	}
}

func TestEmbeddedEngineIsWrittenByHashAndReused(t *testing.T) {
	sum := sha256.Sum256(networkEngineScript)
	want := "network-engine-" + hex.EncodeToString(sum[:])[:12] + ".ps1"
	if !strings.HasPrefix(string(networkEngineScript), "\xef\xbb\xbf# Vendored from Open365 (Apache-2.0) commit ") {
		t.Fatal("engine lost its BOM or provenance line; Windows PowerShell 5.1 needs the BOM to read the Chinese text")
	}
	root := filepath.Join(t.TempDir(), "虾盘 U盘 精灵")
	path, err := ensureNetworkScript(root, networkEngineScript)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "data", "tmp", want) {
		t.Fatalf("path %s, want name %s", path, want)
	}
	if b, _ := os.ReadFile(path); string(b) != string(networkEngineScript) {
		t.Fatal("written content differs from the embedded engine")
	}
	// A matching file is reused, not rewritten.
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err = os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if again, err := ensureNetworkScript(root, networkEngineScript); err != nil || again != path {
		t.Fatalf("%v %s", err, again)
	}
	if info, _ := os.Stat(path); !info.ModTime().Equal(old) {
		t.Fatal("a file with the right hash was rewritten")
	}
	// A tampered file under the same name is replaced.
	if err = os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ensureNetworkScript(root, networkEngineScript); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(networkEngineScript) {
		t.Fatal("tampered engine was not replaced")
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
	// Different content gets a different name, so versions never overwrite each other.
	other, err := ensureNetworkScript(root, []byte("other"))
	if err != nil || other == path {
		t.Fatalf("%v %s", err, other)
	}
}

func TestNetworkRescueHintsForAgent(t *testing.T) {
	zh, en := networkRescueHints(false), networkRescueHints(true)
	if runtime.GOOS != "windows" {
		if zh != "" || en != "" {
			t.Fatal("non-Windows agents must not be told to use a Windows-only tool")
		}
		return
	}
	for _, text := range []string{zh, en} {
		for _, word := range []string{"network.rescue", "next_step", "tools.launch", "reason"} {
			if !strings.Contains(text, word) {
				t.Errorf("hint lacks %q: %s", word, text)
			}
		}
		if strings.Contains(text, "network.repair") {
			t.Errorf("hint mentions a repair action: %s", text)
		}
	}
}
