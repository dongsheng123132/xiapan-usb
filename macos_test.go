package main

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestBundleRootIsFolderBesideApp(t *testing.T) {
	drive := filepath.Join("Volumes", "XIAPAN")
	if root, ok := bundleRoot(filepath.Join(drive, "虾盘.app", "Contents", "MacOS")); !ok || root != drive {
		t.Fatalf("bundle root = %q, %v", root, ok)
	}
	if root, ok := bundleRoot(drive); ok || root != drive {
		t.Fatal("plain folder treated as an app bundle")
	}
	if _, ok := bundleRoot(filepath.Join(drive, "tools", "Contents", "MacOS")); ok {
		t.Fatal("folder without .app treated as an app bundle")
	}
	if got := withoutProcessSerial([]string{"-psn_0_12345", "serve", "--no-open"}); !slices.Equal(got, []string{"serve", "--no-open"}) {
		t.Fatalf("process serial not removed: %v", got)
	}
}

func TestLaunchItemsReportStateWithoutCommands(t *testing.T) {
	overrides := parseLaunchctlDisabled("disabled services = {\n\t\"com.example.off\" => disabled\n\t\"com.example.on\" => enabled\n\t\"com.example.legacy\" => true\n}")
	if !overrides["com.example.off"] || overrides["com.example.on"] || !overrides["com.example.legacy"] {
		t.Fatalf("overrides = %v", overrides)
	}
	row := func(plist string) map[string]any {
		return launchItemRow("file.plist", "当前用户 · LaunchAgents", []byte(plist), overrides)
	}
	cases := []struct{ plist, state, trigger string }{
		{`{"Label":"com.example.off","RunAtLoad":true,"ProgramArguments":["/opt/secret-tool","--token=abc"]}`, "disabled", "登录或开机即运行"},
		{`{"Label":"com.example.on","Disabled":true,"KeepAlive":{"SuccessfulExit":false}}`, "enabled", "登录或开机即运行"},
		{`{"Label":"com.example.plain","Disabled":true,"StartCalendarInterval":{"Hour":3}}`, "disabled", "定时运行"},
		{`{"Label":"com.example.ondemand","MachServices":{"com.example.ondemand":true}}`, "enabled", "按需或条件触发"},
	}
	for _, c := range cases {
		r := row(c.plist)
		if r["state"] != c.state || r["trigger"] != c.trigger {
			t.Fatalf("%s => %v", c.plist, r)
		}
		b, _ := json.Marshal(r)
		if strings.Contains(string(b), "secret-tool") || strings.Contains(string(b), "token") || len(r) != 4 {
			t.Fatalf("launch item exposes more than name, source, state and trigger: %s", b)
		}
	}
	if r := launchItemRow("broken.plist", "系统 · LaunchDaemons", nil, overrides); r["state"] != "unknown" || r["name"] != "broken" {
		t.Fatalf("unreadable plist guessed a state: %v", r)
	}
}

func TestHardwarePortsOmitMACAddresses(t *testing.T) {
	rows := parseHardwarePorts("\nHardware Port: Wi-Fi\nDevice: en0\nEthernet Address: aa:bb:cc:dd:ee:ff\n\nHardware Port: Thunderbolt Bridge\nDevice: bridge0\nEthernet Address: N/A\n")
	if len(rows) != 2 || rows[0]["name"] != "Wi-Fi" || rows[0]["device"] != "en0" || rows[1]["device"] != "bridge0" {
		t.Fatalf("ports = %v", rows)
	}
	b, _ := json.Marshal(rows)
	if strings.Contains(string(b), "aa:bb") {
		t.Fatal("hardware address exposed")
	}
}

func TestSystemToolsStayOnTheirPlatform(t *testing.T) {
	native := map[string]bool{}
	for _, tool := range nativeTools() {
		native[tool.ID] = true
	}
	enum := maintenanceToolSchema()["properties"].(map[string]any)["tool_id"].(map[string]any)["enum"].([]string)
	for _, tool := range append(append([]BuiltinTool{}, builtinTools...), macTools...) {
		if allowed := requireBuiltinTool(tool.ID) == nil; allowed != native[tool.ID] || slices.Contains(enum, tool.ID) != native[tool.ID] {
			t.Fatalf("%s offered on %s: %v", tool.ID, runtime.GOOS, allowed)
		}
	}
	for _, tool := range macTools {
		if !path.IsAbs(tool.Program) || !strings.HasSuffix(tool.Program, ".app") {
			t.Fatalf("mac tool %s is not a fixed app bundle", tool.ID)
		}
		for _, a := range tool.Args {
			if !strings.HasPrefix(a, "x-apple.systempreferences:") {
				t.Fatalf("mac tool %s passes an unexpected argument", tool.ID)
			}
		}
	}
	hints := agentToolHints()
	if runtime.GOOS == "darwin" && (strings.Contains(hints, "windirstat") || strings.Contains(hints, "device-manager")) {
		t.Fatal("Mac agent offered Windows tools")
	}
}

func TestPlatformManifestSelection(t *testing.T) {
	b, err := platformCatalog("picoclaw")
	if err != nil {
		t.Fatal(err)
	}
	var p PicoManifest
	if err = json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	own := runtime.GOOS + "/" + runtime.GOARCH
	if _, err = assets.ReadFile("catalog/picoclaw." + runtime.GOOS + "-" + runtime.GOARCH + ".json"); err == nil && p.Platform != own {
		t.Fatalf("platform manifest not preferred: %s", p.Platform)
	}
	if p.Platform != own && picoManifest().Platform == own {
		t.Fatal("manifest selection is inconsistent")
	}
	if got := hostLabel(SystemInfo{OS: "darwin", OSVersion: "15.6.1", Arch: "arm64"}); got != "macOS 15.6.1 / arm64" {
		t.Fatal(got)
	}
	if got := hostLabel(SystemInfo{OS: "windows", Arch: "amd64"}); got != "windows/amd64" {
		t.Fatal(got)
	}
}

func TestResourceScanSkipsFinderMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "models")
	os.MkdirAll(dir, 0700)
	for _, name := range []string{"model.gguf", "._model.gguf", ".DS_Store"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600)
	}
	scan, err := scanResources(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Files) != 1 || scan.Files[0].Name != "model.gguf" {
		t.Fatalf("metadata listed as resources: %+v", scan.Files)
	}
}
