package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestBuiltinPlatformCatalog(t *testing.T) {
	for _, tc := range []struct {
		platform string
		count    int
	}{{"windows", 9}, {"darwin", 5}, {"linux", 0}} {
		t.Run(tc.platform, func(t *testing.T) {
			tools := builtinToolsForPlatform(tc.platform)
			if len(tools) != tc.count {
				t.Fatalf("system tools = %d, want %d", len(tools), tc.count)
			}
			seen := map[string]bool{}
			for _, tool := range tools {
				if tool.Platform != tc.platform || seen[tool.ID] {
					t.Fatalf("wrong platform or duplicate ID: %+v", tool)
				}
				seen[tool.ID] = true
			}
		})
	}
	result, err := toolCatalog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rows := result.(map[string]any)["builtin_tools"].([]any)
	if len(rows) != len(builtinToolsForPlatform(runtime.GOOS)) {
		t.Fatalf("catalog includes system tools from another platform: %#v", rows)
	}
	for _, row := range rows {
		if row.(map[string]any)["platform"] != runtime.GOOS {
			t.Fatalf("cross-platform system row: %#v", row)
		}
	}
}

func TestMacBuiltinExactAppCommands(t *testing.T) {
	regular, err := os.Stat("maintenance_tools.go")
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range builtinToolsForPlatform("darwin") {
		t.Run(tool.ID, func(t *testing.T) {
			if tool.Program != "" || len(tool.Args) != 0 || len(tool.AppPaths) < 1 {
				t.Fatalf("Mac tool must only select a fixed app bundle: %+v", tool)
			}
			for _, candidate := range tool.AppPaths {
				if (!strings.HasPrefix(candidate, "/System/") && !strings.HasPrefix(candidate, "/Applications/Utilities/")) || !strings.HasSuffix(candidate, ".app") || strings.Contains(candidate, "..") {
					t.Fatalf("not a fixed system app path: %q", candidate)
				}
				stat := func(path string) (os.FileInfo, error) {
					switch path {
					case "/usr/bin/open":
						return regular, nil
					case candidate:
						return directory, nil
					default:
						return nil, os.ErrNotExist
					}
				}
				program, args, ready := builtinCommandForPlatform(tool, "darwin", "", stat)
				if !ready || program != "/usr/bin/open" || !reflect.DeepEqual(args, []string{candidate}) {
					t.Fatalf("command = %q %#v %v; want exact app %q", program, args, ready, candidate)
				}
			}
			// No installed bundle, an app-shaped regular file, or missing system
			// opener must remain unavailable rather than searching PATH or by name.
			for _, mode := range []string{"missing-app", "file-not-bundle", "missing-open"} {
				stat := func(path string) (os.FileInfo, error) {
					if path == "/usr/bin/open" && mode != "missing-open" {
						return regular, nil
					}
					if mode == "file-not-bundle" {
						return regular, nil
					}
					return nil, os.ErrNotExist
				}
				if _, _, ready := builtinCommandForPlatform(tool, "darwin", "", stat); ready {
					t.Fatalf("%s unexpectedly ready", mode)
				}
			}
			if _, _, ready := builtinCommandForPlatform(tool, "windows", "", func(string) (os.FileInfo, error) {
				t.Fatal("platform mismatch must be rejected before file access")
				return nil, os.ErrNotExist
			}); ready {
				t.Fatal("Mac app available on Windows")
			}
		})
	}
}

func TestWindowsBuiltinFixedCommands(t *testing.T) {
	dir := t.TempDir()
	for _, tool := range builtinToolsForPlatform("windows") {
		if err := os.WriteFile(filepath.Join(dir, tool.Program), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, arg := range tool.Args {
			if strings.HasSuffix(arg, ".msc") || strings.HasSuffix(arg, ".cpl") {
				if err := os.WriteFile(filepath.Join(dir, arg), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		program, args, ready := builtinCommandForPlatform(tool, "windows", dir, os.Stat)
		if !ready || program != filepath.Join(dir, tool.Program) || len(args) != len(tool.Args) {
			t.Fatalf("%s: unexpected command %q %#v, ready %v", tool.ID, program, args, ready)
		}
		for i, arg := range tool.Args {
			expected := arg
			if strings.HasSuffix(arg, ".msc") || strings.HasSuffix(arg, ".cpl") {
				expected = filepath.Join(dir, arg)
			}
			if args[i] != expected {
				t.Fatalf("%s: argument %q, want %q", tool.ID, args[i], expected)
			}
		}
		if err := os.Remove(filepath.Join(dir, tool.Program)); err != nil {
			t.Fatal(err)
		}
		if _, _, ready := builtinCommandForPlatform(tool, "windows", dir, os.Stat); ready {
			t.Fatalf("%s: missing executable unexpectedly ready", tool.ID)
		}
	}
}

func TestSystemToolLaunchRequiresKnownIDAndConfirmation(t *testing.T) {
	for _, id := range []string{"/usr/bin/open", "mac-disk-utility --args erase", "arbitrary-shell"} {
		if _, err := launchTool(context.Background(), id, true); err == nil {
			t.Fatalf("unknown ID accepted: %q", id)
		}
	}
	for _, tool := range builtinTools {
		if _, err := launchTool(context.Background(), tool.ID, false); err == nil {
			t.Fatalf("unconfirmed system launch accepted: %q", tool.ID)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := launchTool(ctx, "mac-disk-utility", true); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled launch = %v", err)
	}
}
