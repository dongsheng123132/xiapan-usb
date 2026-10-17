package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func writePidRecord(t *testing.T, root string, rec picoPidRecord) {
	t.Helper()
	dir := filepath.Join(root, "data", "pico")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(dir, ".picoclaw.pid"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReadPicoPidRecordRejectsInvalid(t *testing.T) {
	root := t.TempDir()
	for name, rec := range map[string]picoPidRecord{
		"zero pid":    {PID: 0, Port: 56919, Host: "127.0.0.1"},
		"bad port":    {PID: 1234, Port: 70000, Host: "127.0.0.1"},
		"bad host":    {PID: 1234, Port: 56919, Host: "0.0.0.0"},
		"zero port":   {PID: 1234, Port: 0, Host: "127.0.0.1"},
	} {
		writePidRecord(t, root, rec)
		if _, ok := readPicoPidRecord(root); ok {
			t.Fatalf("accepted invalid record %s", name)
		}
	}
	writePidRecord(t, root, picoPidRecord{PID: 1234, Port: 56919, Host: "127.0.0.1"})
	if _, ok := readPicoPidRecord(root); !ok {
		t.Fatal("rejected valid record")
	}
}

func TestHealthPID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"pid":1234}`))
	}))
	defer server.Close()
	addr := strings.TrimPrefix(server.URL, "http://")
	if got := healthPID(addr); got != 1234 {
		t.Fatalf("health pid = %d, want 1234", got)
	}
}

func TestHealthPIDRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	if got := healthPID(addr); got != 0 {
		t.Fatalf("health pid = %d, want 0", got)
	}
}

func TestPicoTokenHandshake(t *testing.T) {
	upgrader := websocket.Upgrader{Subprotocols: []string{"token.good"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/pico/ws", func(w http.ResponseWriter, r *http.Request) {
		// The real gateway fails the handshake when the requested subprotocol
		// does not match; mirror that strictly (gorilla v1.4.2 omits the
		// response header instead of rejecting).
		if r.Header.Get("Sec-WebSocket-Protocol") != "token.good" {
			http.Error(w, "unexpected subprotocol", http.StatusBadRequest)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			conn.Close()
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	addr := strings.TrimPrefix(server.URL, "http://")
	if !picoTokenHandshake("good", addr) {
		t.Fatal("good token handshake failed")
	}
	if picoTokenHandshake("bad", addr) {
		t.Fatal("bad token handshake accepted")
	}
	if picoTokenHandshake("", addr) {
		t.Fatal("empty token handshake accepted")
	}
}

func TestPicoChannelToken(t *testing.T) {
	root := t.TempDir()
	if got := picoChannelToken(root); got != "" {
		t.Fatalf("missing security file returned token %q", got)
	}
	dir := filepath.Join(root, "data", "pico")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	yml := "channel_list:\n  pico:\n    settings:\n      token: abc123\n"
	if err := os.WriteFile(filepath.Join(dir, ".security.yml"), []byte(yml), 0600); err != nil {
		t.Fatal(err)
	}
	if got := picoChannelToken(root); got != "abc123" {
		t.Fatalf("token = %q, want abc123", got)
	}
}

func TestOrphanOwnershipRejectsWrongPID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"pid":1234}`))
	}))
	defer server.Close()
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	root := t.TempDir()
	rec := picoPidRecord{PID: 9999, Port: port, Host: host}
	writePidRecord(t, root, rec)
	if picoOrphanOwnedByThisDrive(root, rec) {
		t.Fatal("accepted ownership when health pid differs")
	}
}

func TestStopVerifiedPicoOrphanAlreadyGone(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	host, portStr, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portStr)
	rec := picoPidRecord{PID: 0xFFFFFFF, Port: port, Host: host}
	if err := stopVerifiedPicoOrphan(rec); err != nil {
		t.Fatalf("already-gone orphan should not error: %v", err)
	}
}

func TestPicoProcessImagePathSelf(t *testing.T) {
	path, ok := picoProcessImagePath(os.Getpid())
	if !ok || path == "" {
		t.Fatalf("self image path unavailable: %q", path)
	}
}

func TestPicoProcessIsOursRejectsForeign(t *testing.T) {
	// Our own test process lives outside every pico runtime root.
	if picoProcessIsOurs(t.TempDir(), os.Getpid()) {
		t.Fatal("accepted foreign process as ours")
	}
}

func TestIsPathWithin(t *testing.T) {
	base := filepath.Join(t.TempDir(), "runtime")
	inside := filepath.Join(base, "picoclaw", "picoclaw")
	sibling := filepath.Join(filepath.Dir(base), "runtime-other", "picoclaw")
	cases := map[string]struct {
		path string
		want bool
	}{
		"same":      {base, true},
		"inside":    {inside, true},
		"sibling":   {sibling, false},
		"prefix":    {base + "-x", false},
		"escape":    {filepath.Join(base, "..", "..", "etc"), false},
	}
	for name, tc := range cases {
		if got := isPathWithin(base, tc.path); got != tc.want {
			t.Fatalf("%s: isPathWithin(%q, %q) = %v, want %v", name, base, tc.path, got, tc.want)
		}
	}
}
