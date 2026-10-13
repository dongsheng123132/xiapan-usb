package main

import (
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPicoCredentialPreservesNativeConfiguration(t *testing.T) {
	root := t.TempDir()
	if err := applyPicoCredential(root, "test-key-one"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := readPicoMap(root, "config.json", false)
	picoMap(picoMap(cfg, "agents"), "defaults")["model_name"] = "my-native-model"
	cfg["model_list"] = append(cfg["model_list"].([]any), map[string]any{"model_name": "my-native-model", "model": "other", "provider": "openai"})
	picoMap(picoMap(cfg, "tools"), "exec")["enabled"] = false
	if err := writePicoMap(root, "config.json", cfg, false); err != nil {
		t.Fatal(err)
	}
	security, _ := readPicoMap(root, ".security.yml", true)
	picoMap(security, "model_list")["my-native-model:0"] = map[string]any{"api_keys": []string{"other-key"}}
	if err := writePicoMap(root, ".security.yml", security, true); err != nil {
		t.Fatal(err)
	}
	if err := applyPicoCredential(root, "test-key-one"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = readPicoMap(root, "config.json", false)
	if picoMap(picoMap(cfg, "agents"), "defaults")["model_name"] != "my-native-model" {
		t.Fatal("unchanged bridge replaced native default")
	}
	if picoMap(picoMap(cfg, "tools"), "exec")["enabled"] != false {
		t.Fatal("re-enabled user-disabled tool")
	}
	if err := applyPicoCredential(root, "test-key-two"); err != nil {
		t.Fatal(err)
	}
	cfg, _ = readPicoMap(root, "config.json", false)
	security, _ = readPicoMap(root, ".security.yml", true)
	if len(cfg["model_list"].([]any)) != 2 || picoMap(security, "model_list")["my-native-model:0"] == nil {
		t.Fatal("lost native model or secret")
	}
	if picoMap(picoMap(cfg, "tools"), "exec")["enabled"] != false {
		t.Fatal("credential rotation changed tool settings")
	}
}

func TestPinnedGatewayIntegration(t *testing.T) {
	if os.Getenv("XIAPAN_PICO_INTEGRATION") != "1" {
		t.Skip("set XIAPAN_PICO_INTEGRATION=1 with pinned runtime prepared")
	}
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	slot, ok := picoManifest().currentSlot()
	if !ok {
		t.Skip("runtime not provided")
	}
	for _, rel := range []string{slot.Path, slot.LauncherPath} {
		if rel == "" {
			t.Fatal("launcher not locked")
		}
		src, err := os.Open(filepath.FromSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(dst), 0700)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(out, src)
		out.Close()
		src.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	requests := 0
	auths := []string{}
	seenPrior := false
	seenWrite := false
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []any{map[string]any{"id": "fixture-model", "object": "model"}}})
			return
		}
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			http.Error(w, "bad", 400)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		requests++
		auths = append(auths, r.Header.Get("Authorization"))
		messages, _ := input["messages"].([]any)
		raw, _ := json.Marshal(messages)
		if strings.Contains(string(raw), "fixture-ok") {
			seenPrior = true
		}
		tools, _ := input["tools"].([]any)
		for _, entry := range tools {
			b, _ := json.Marshal(entry)
			if strings.Contains(string(b), "\"write_file\"") {
				seenWrite = true
			}
		}
		var message map[string]any
		finish := "stop"
		if requests == 1 {
			message = map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": "write-fixture", "type": "function", "function": map[string]any{"name": "write_file", "arguments": `{"path":"gateway-fixture.txt","content":"native-write-ok"}`}}}}
			finish = "tool_calls"
		} else {
			message = map[string]any{"role": "assistant", "content": "fixture-ok"}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "fixture-response", "object": "chat.completion", "model": "fixture-model", "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20}})
	}))
	defer model.Close()
	setting := ModelSettings{Revision: "fixture-1", Source: "custom", Model: "fixture-model", BaseURL: model.URL + "/v1", APIKey: "fixture-key-1"}
	b, _ := json.Marshal(setting)
	if err := replaceLocalFile(root, []string{"data", "settings", "model.json"}, b); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer stopOwnedPicoProcesses(root)
	session := &ChatSession{ID: uniqueID(), Messages: []ChatMessage{}, Pending: []Proposal{}}
	text, err := picoConversation(ctx, root, session, "Write the fixture file, then confirm.", "en")
	if err != nil {
		t.Fatal(err)
	}
	if text != "fixture-ok" {
		t.Fatalf("answer %q", text)
	}
	contents, err := os.ReadFile(filepath.Join(root, "data", "pico", "workspace", "gateway-fixture.txt"))
	if err != nil || string(contents) != "native-write-ok" {
		t.Fatalf("native write not executed: %v", err)
	}
	g := picoRuntime(root)
	pid := g.gateway.cmd.Process.Pid
	text, err = picoConversation(ctx, root, session, "Continue this conversation.", "en")
	if err != nil || text != "fixture-ok" {
		t.Fatalf("second turn: %q %v", text, err)
	}
	if g.gateway.cmd.Process.Pid != pid {
		t.Fatal("gateway restarted each turn")
	}
	mu.Lock()
	prior, toolsEnabled := seenPrior, seenWrite
	mu.Unlock()
	if !prior || !toolsEnabled {
		t.Fatalf("history=%v native tools=%v", prior, toolsEnabled)
	}
	setting.Revision = "fixture-2"
	setting.APIKey = "fixture-key-2"
	b, _ = json.Marshal(setting)
	replaceLocalFile(root, []string{"data", "settings", "model.json"}, b)
	if err = applyPicoCredential(root, ""); err != nil {
		t.Fatal(err)
	}
	// Official file watching is debounced; the next completed request must use the new key.
	time.Sleep(1500 * time.Millisecond)
	_, err = picoConversation(ctx, root, session, "Use the updated credential.", "en")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	lastAuth := auths[len(auths)-1]
	mu.Unlock()
	if lastAuth != "Bearer fixture-key-2" {
		t.Fatalf("gateway used stale credential: %q", lastAuth)
	}
	ui, err := picoWebUIStart(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	address := ui.(map[string]any)["url"].(string)
	client := localHTTP(12 * time.Second)
	req, _ := http.NewRequest(http.MethodPost, address+"/api/auth/setup", strings.NewReader(`{"password":"fixture-panel-password","confirm":"fixture-panel-password"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("official setup %d %s", resp.StatusCode, body)
	}
	req, _ = http.NewRequest(http.MethodPost, address+"/api/auth/login", strings.NewReader(`{"password":"fixture-panel-password"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("official login failed: %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	request := func(method, path string) (int, []byte) {
		req, _ := http.NewRequest(method, address+path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}
	status, body := request("GET", "/api/gateway/status")
	if status != 200 || !strings.Contains(string(body), "running") {
		t.Fatalf("launcher did not attach: %d %s", status, body)
	}
	status, body = request("GET", "/api/sessions")
	if status != 200 || !strings.Contains(string(body), session.ID) {
		t.Fatalf("official UI cannot see simple conversation: %d %s", status, body)
	}
	status, _ = request("POST", "/api/update")
	if status >= 200 && status < 300 {
		t.Fatal("official update endpoint still enabled")
	}
	output, err := exec.Command(filepath.Join(root, filepath.FromSlash(slot.Path)), "update", "--help").CombinedOutput()
	if err == nil {
		t.Fatalf("update CLI still enabled: %s", output)
	}
	// A native Restart must keep its panel alive and hand its replacement
	// core back to the simple UI, rather than spawning another instance.
	restartNative := func() int {
		t.Helper()
		panel := g.launcher
		status, body := request("POST", "/api/gateway/restart")
		if status != http.StatusOK {
			t.Fatalf("native restart failed: %d %s", status, body)
		}
		var result struct {
			PID int `json:"pid"`
		}
		if err := json.Unmarshal(body, &result); err != nil || result.PID <= 0 {
			t.Fatalf("native restart response: %s", body)
		}
		t.Logf("Restart returned replacement=%d original=%d original_wait_alive=%v original_os_alive=%v launcher=%d", result.PID, g.gateway.cmd.Process.Pid, picoAlive(g.gateway), picoProcessExists(g.gateway.cmd.Process.Pid), g.launcher.cmd.Process.Pid)
		answer, err := picoConversation(ctx, root, session, "Continue after the official Restart.", "en")
		if err != nil || answer != "fixture-ok" {
			t.Fatalf("simple UI after native restart: %q %v (verified=%d detected=%d original_wait_alive=%v original_os_alive=%v replacement_alive=%v)", answer, err, g.verifiedGatewayPID, g.gatewayPIDLocked(), picoAlive(g.gateway), picoProcessExists(g.gateway.cmd.Process.Pid), picoProcessExists(result.PID))
		}
		g.mu.Lock()
		actualPID := g.gatewayPIDLocked()
		g.mu.Unlock()
		if actualPID != result.PID || g.launcher != panel || !picoAlive(panel) || g.launcherURL != address {
			t.Fatalf("native restart was not reused: wanted PID %d, got %d", result.PID, actualPID)
		}
		state, err := picoGatewayInspect(root)
		if err != nil || state.(map[string]any)["running"] != true {
			t.Fatalf("native restart status: %#v %v", state, err)
		}
		status, body = request("GET", "/api/gateway/status")
		var nativeStatus struct {
			PID int `json:"pid"`
		}
		json.Unmarshal(body, &nativeStatus)
		if status != http.StatusOK || nativeStatus.PID != result.PID {
			t.Fatalf("panel lost replacement core: %d %s", status, body)
		}
		return result.PID
	}
	waitProcessGone := func(pid int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for picoProcessExists(pid) && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
		if picoProcessExists(pid) {
			t.Fatalf("launcher replacement PID %d survived shutdown", pid)
		}
	}
	firstReplacement := restartNative()
	// Rotate while the advanced UI is already open, then submit through its
	// authenticated WebSocket proxy and prove it uses the new credential.
	setting.Revision = "fixture-3"
	setting.APIKey = "fixture-key-3"
	b, _ = json.Marshal(setting)
	if err = replaceLocalFile(root, []string{"data", "settings", "model.json"}, b); err != nil {
		t.Fatal(err)
	}
	if err = applyPicoCredential(root, ""); err != nil {
		t.Fatal(err)
	}
	oldCore, oldUI := g.gateway, g.launcher
	if err = syncOwnedPicoCredentials(ctx, root); err != nil {
		t.Fatal(err)
	}
	if picoAlive(oldCore) || picoAlive(oldUI) {
		t.Fatal("credential synchronization left an old process alive")
	}
	waitProcessGone(firstReplacement)
	if g.launcherURL != address {
		t.Fatal("advanced UI address changed after credential rotation")
	}
	req, _ = http.NewRequest(http.MethodPost, address+"/api/auth/login", strings.NewReader(`{"password":"fixture-panel-password"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("panel password did not survive restart: %d", resp.StatusCode)
	}
	cookies = resp.Cookies()
	headers := http.Header{}
	for _, cookie := range cookies {
		headers.Add("Cookie", cookie.Name+"="+cookie.Value)
	}
	wsURL := "ws" + strings.TrimPrefix(address, "http") + "/pico/ws?session_id=" + session.ID
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	err = conn.WriteJSON(map[string]any{"type": "message.send", "id": "advanced-fixture", "payload": map[string]any{"content": "Continue from the advanced UI."}})
	if err != nil {
		t.Fatal(err)
	}
	for {
		var frame picoWireMessage
		if err = conn.ReadJSON(&frame); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if frame.Payload["content"] == "fixture-ok" {
			break
		}
	}
	conn.Close()
	mu.Lock()
	lastAuth = auths[len(auths)-1]
	mu.Unlock()
	if lastAuth != "Bearer fixture-key-3" {
		t.Fatal("advanced UI used old credential")
	}
	lastReplacement := restartNative()
	core, launcher := g.gateway, g.launcher
	stopOwnedPicoProcesses(root)
	waitProcessGone(lastReplacement)
	if picoAlive(core) || picoAlive(launcher) {
		t.Fatal("owned processes survived exit")
	}
	t.Logf("verified persistent PID, native write, shared history, credential synchronization (both UIs), launcher auth, native Restart handoff (both UIs), disabled updates and cleanup on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func TestPicoGatewayChatStreamingAndCancellation(t *testing.T) {
	stopSeen := make(chan struct{}, 1)
	initialSeen := make(chan struct{}, 1)
	up := websocket.Upgrader{Subprotocols: []string{"token.fixture"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var input map[string]any
		if conn.ReadJSON(&input) != nil {
			return
		}
		if r.URL.Query().Get("session_id") == "cancel" {
			initialSeen <- struct{}{}
			if conn.ReadJSON(&input) == nil {
				if p, ok := input["payload"].(map[string]any); ok && p["content"] == "/stop" {
					stopSeen <- struct{}{}
				}
			}
			return
		}
		for _, frame := range []picoWireMessage{
			{Type: "typing.start"},
			{Type: "message.create", Payload: map[string]any{"message_id": "thought", "kind": "thought", "content": "private reasoning"}},
			{Type: "message.create", Payload: map[string]any{"message_id": "answer", "content": "partial"}},
			{Type: "message.update", Payload: map[string]any{"message_id": "answer", "content": "complete answer"}},
			{Type: "typing.stop"},
		} {
			if conn.WriteJSON(frame) != nil {
				return
			}
		}
	}))
	defer server.Close()
	g := &picoGatewayRuntime{address: strings.TrimPrefix(server.URL, "http://"), token: "fixture"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	answer, err := picoGatewayChat(ctx, g, "stream", "fixture")
	if err != nil || answer != "complete answer" {
		t.Fatalf("streamed answer=%q err=%v", answer, err)
	}
	canceled, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, err := picoGatewayChat(canceled, g, "cancel", "fixture"); done <- err }()
	select {
	case <-initialSeen:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	select {
	case <-stopSeen:
	case <-ctx.Done():
		t.Fatal("native /stop was not sent")
	}
	if err = <-done; err == nil {
		t.Fatal("canceled request reported success")
	}
}
