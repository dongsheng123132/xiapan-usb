package main

import (
	"context"
	"strings"
	"testing"
)

func TestDarwinSystemInspectReadsHostAndVolumes(t *testing.T) {
	info, err := inspectSystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if info.OSVersion == "" || info.MemoryBytes == 0 || info.AvailableMemoryBytes == 0 || info.AvailableMemoryBytes > info.MemoryBytes {
		t.Fatalf("host details missing: %+v", info)
	}
	found := false
	for _, v := range info.Volumes {
		if v.SizeBytes == 0 || v.FreeBytes > v.SizeBytes {
			t.Fatalf("volume sizes invalid: %+v", v)
		}
		found = found || v.Path == "/"
	}
	if !found {
		t.Fatalf("startup volume missing: %+v", info.Volumes)
	}
}

func TestDarwinStartupInventoryIsReadOnly(t *testing.T) {
	r := runAction(context.Background(), t.TempDir(), "startup.inspect", map[string]any{})
	if !r.OK {
		t.Fatal(r.Error)
	}
	for _, item := range r.Data.(map[string]any)["items"].([]map[string]any) {
		if s := item["state"]; s != "enabled" && s != "disabled" && s != "unknown" {
			t.Fatalf("unexpected state: %v", item)
		}
	}
}

func TestDarwinToolsOpenThroughLaunchServices(t *testing.T) {
	for _, tool := range macTools {
		program, args, ready := builtinCommand(tool)
		if !ready {
			continue
		}
		if program != "/usr/bin/open" || len(args) != 1 || (args[0] != tool.Program && !strings.HasPrefix(args[0], "x-apple.systempreferences:")) {
			t.Fatalf("%s launches %s %v", tool.ID, program, args)
		}
	}
	if _, ok := translocatedOrigin("/private/var/folders/xx/T/AppTranslocation/0000/d/虾盘.app/Contents/MacOS/xiapan"); ok {
		t.Fatal("unmounted translocation path resolved")
	}
	if _, ok := translocatedOrigin("/Volumes/XIAPAN/虾盘.app/Contents/MacOS/xiapan"); ok {
		t.Fatal("ordinary path treated as translocated")
	}
}
