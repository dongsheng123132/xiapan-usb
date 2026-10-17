package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// A previous wrapper of the same drive may die while its native gateway keeps
// running. The state lock already proves no wrapper is active for this drive,
// so such a service can only be an orphan of this drive. Before touching it we
// re-confirm live identity, so a reused PID, a foreign process on the recorded
// port, or a service holding an unrelated token can never be terminated:
//
//  1. /health must report the pid recorded in .picoclaw.pid;
//  2. the process image must live under our runtime cache or the portable
//     runtime directory;
//  3. the Pico channel token from .security.yml must complete the websocket
//     handshake (the token inside .picoclaw.pid is not the auth credential).
//
// The pid file alone is never authority to terminate (PID reuse / spoofing).

type picoPidRecord struct {
	PID     int    `json:"pid"`
	Token   string `json:"token"`
	Version string `json:"version"`
	Port    int    `json:"port"`
	Host    string `json:"host"`
}

// readPicoPidRecord loads and shape-checks the pid file the native gateway
// writes on the drive.
func readPicoPidRecord(root string) (picoPidRecord, bool) {
	path, err := writablePath(root, "data", "pico", ".picoclaw.pid")
	if err != nil {
		return picoPidRecord{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 || len(b) > 4096 {
		return picoPidRecord{}, false
	}
	var rec picoPidRecord
	if json.Unmarshal(b, &rec) != nil || rec.PID <= 0 || rec.Port <= 0 || rec.Port >= 65536 || rec.Host != "127.0.0.1" {
		return picoPidRecord{}, false
	}
	return rec, true
}

func picoGatewayAddress(rec picoPidRecord) string {
	return net.JoinHostPort(rec.Host, strconv.Itoa(rec.Port))
}

// healthPID probes /health and returns the pid the service reports. It
// returns 0 when the port is unreachable, the status is not 200, or the body
// is malformed.
func healthPID(address string) int {
	resp, err := localHTTP(400 * time.Millisecond).Get("http://" + address + "/health")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var h struct {
		PID int `json:"pid"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&h) != nil {
		return 0
	}
	return h.PID
}

// picoChannelToken reads the Pico channel token from .security.yml, the same
// credential the wrapper writes at launch and the gateway authenticates with.
func picoChannelToken(root string) string {
	security, err := readPicoMap(root, ".security.yml", true)
	if err != nil {
		return ""
	}
	settings := picoMap(picoMap(picoMap(security, "channel_list"), "pico"), "settings")
	token, _ := settings["token"].(string)
	return token
}

// picoTokenHandshake verifies a token completes the gateway websocket
// handshake. Authentication uses the separate, unchanged Pico channel token;
// this handshake does not submit model work.
func picoTokenHandshake(token, address string) bool {
	if token == "" {
		return false
	}
	dialer := websocket.Dialer{HandshakeTimeout: time.Second, Subprotocols: []string{"token." + token}, Proxy: nil}
	conn, _, err := dialer.DialContext(context.Background(), "ws://"+address+"/pico/ws?session_id=xiapan-orphan-"+uniqueID(), nil)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// picoOrphanOwnedByThisDrive reports whether the recorded service is really
// this drive's gateway: recorded pid, image path and channel token all match.
func picoOrphanOwnedByThisDrive(root string, rec picoPidRecord) bool {
	address := picoGatewayAddress(rec)
	if healthPID(address) != rec.PID {
		return false
	}
	if !picoProcessIsOurs(root, rec.PID) {
		return false
	}
	return picoTokenHandshake(picoChannelToken(root), address)
}

// stopVerifiedPicoOrphan terminates a service that already passed
// picoOrphanOwnedByThisDrive and waits until its port stops answering. The
// stale pid file is removed by the caller on success.
func stopVerifiedPicoOrphan(rec picoPidRecord) error {
	address := picoGatewayAddress(rec)
	if err := terminatePicoProcess(rec.PID); err != nil {
		// Already gone is fine.
		if healthPID(address) == 0 {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if healthPID(address) == 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("服务未在预期时间内退出")
}

// isPathWithin reports whether path equals base or lies under it.
func isPathWithin(base, path string) bool {
	base = filepath.Clean(base)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}
