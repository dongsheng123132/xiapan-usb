package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNativeMCPEnforcesCapabilityAndConfirmation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := ChatSession{ID: uniqueID(), Messages: []ChatMessage{}, Pending: []Proposal{}}
	config, stop, err := startPicoMCP(ctx, t.TempDir(), &s)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	url := config["url"].(string)
	auth := config["headers"].(map[string]string)["Authorization"]
	call := func(body, token, origin string) (int, map[string]any) {
		r, _ := http.NewRequest("POST", url, strings.NewReader(body))
		r.Header.Set("Authorization", token)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		resp, err := localHTTP(3 * time.Second).Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var data map[string]any
		json.NewDecoder(resp.Body).Decode(&data)
		return resp.StatusCode, data
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"maintenance_action","arguments":{"action":"report.create","confirmed":true}}}`
	if status, _ := call(body, "", ""); status != 403 {
		t.Fatal("unauthenticated maintenance accepted")
	}
	if status, _ := call(body, auth, "https://external.example"); status != 403 {
		t.Fatal("external origin accepted")
	}
	if len(s.Pending) != 0 {
		t.Fatal("untrusted request changed state")
	}
	if status, data := call(body, auth, ""); status != 200 || data["error"] != nil {
		t.Fatalf("tool failed: %d %#v", status, data)
	}
	if len(s.Pending) != 1 || len(s.Messages) != 1 {
		t.Fatal("model did not form exactly one proposal")
	}
	call(body, auth, "")
	if len(s.Pending) != 1 || len(s.Messages) != 1 {
		t.Fatal("duplicate request ran again")
	}
	// Even a malicious model's confirmed field cannot authorize a real write.
	if s.Pending[0].Input["confirmed"] != nil {
		t.Fatal("model authorization entered proposal")
	}
	call(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"maintenance_action","arguments":{"action":"wallet.rotate","confirmed":true}}}`, auth, "")
	if len(s.Pending) != 1 {
		t.Fatal("wallet operation exposed to model")
	}
	cancel()
	if status, _ := call(body, auth, ""); status != 410 {
		t.Fatal("expired capability accepted")
	}
}

func TestToolLaunchRejectsUnregisteredAndUnconfirmed(t *testing.T) {
	if _, err := launchTool(context.Background(), "device-manager", false); err == nil {
		t.Fatal("unconfirmed GUI launch accepted")
	}
	if _, err := launchTool(context.Background(), "arbitrary-shell", true); err == nil {
		t.Fatal("unregistered program accepted")
	}
}
