package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSlowUSBTimeoutBudgets(t *testing.T) {
	root := t.TempDir()
	for _, c := range []struct {
		action string
		engine string
		want   time.Duration
	}{
		{"system.inspect", "", 60 * time.Second},
		{"system.inspect", "local", 60 * time.Second},
		{"models.start", "", 240 * time.Second},
		{"chat.send", "pico", 60 * time.Second},
		{"chat.send", "cloud", 60 * time.Second},
		{"chat.send", "guide", 60 * time.Second},
		{"chat.send", "", 60 * time.Second},
		{"chat.send", "local", 240 * time.Second},
	} {
		if got := actionTimeout(root, c.action, map[string]any{"engine": c.engine}); got != c.want {
			t.Fatalf("%s/%s budget %s, want %s", c.action, c.engine, got, c.want)
		}
	}
	markOfflineTestRoot(t, root)
	if actionTimeout(root, "chat.send", nil) != 60*time.Second {
		t.Fatal("missing-model guide should retain the ordinary budget")
	}
	m, err := aiManifest()
	if err == nil && m.Ready && m.Platform == runtime.GOOS+"/"+runtime.GOARCH {
		for _, rel := range []string{m.ModelPath, m.EnginePath} {
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("inventory fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if actionTimeout(root, "chat.send", nil) != 240*time.Second {
			t.Fatal("default bundled offline question did not receive startup budget")
		}
	}
	server, err := newServer(root, "http://127.0.0.1:19876")
	if err != nil {
		t.Fatal(err)
	}
	if server.WriteTimeout < 245*time.Second {
		t.Fatal("HTTP response deadline would truncate a permitted local action")
	}
}

type cancellingHashReader struct {
	cancel context.CancelFunc
	reads  int
}

func (r *cancellingHashReader) Read(p []byte) (int, error) {
	r.reads++
	n := copy(p, "model bytes")
	r.cancel()
	return n, nil
}

func TestModelHashStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &cancellingHashReader{cancel: cancel}
	_, err := io.Copy(io.Discard, contextResourceReader{ctx: ctx, reader: source})
	if !errors.Is(err, context.Canceled) || source.reads != 1 {
		t.Fatalf("cancelled model hash kept reading: %v, reads=%d", err, source.reads)
	}
	root := t.TempDir()
	if _, err := verifyResourceContext(ctx, root, "models/missing.gguf", "unused"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled verification still performed filesystem work: %v", err)
	}
	if _, err := startModel(ctx, root, map[string]any{"confirmed": true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled startup did not stop before resources/processes: %v", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("cancelled startup created state")
	}
}
