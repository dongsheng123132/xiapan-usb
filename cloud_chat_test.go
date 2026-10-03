package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const fixtureKey = "sk-fixture-wallet-no-real-credential"

func fixtureCloud(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	t.Setenv("UCLAW_API_BASE_URL", s.URL)
	return s
}
func usageReply(w http.ResponseWriter) {
	w.Write([]byte(`{"success":true,"data":{"total_available":12500,"total_used":300}}`))
}

func TestWalletEnsureConcurrentUsesOneBind(t *testing.T) {
	var binds atomic.Int32
	fixtureCloud(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device/bind":
			binds.Add(1)
			json.NewEncoder(w).Encode(map[string]string{"apiKey": fixtureKey, "walletId": "fixture-wallet"})
		case "/api/usage/token/":
			usageReply(w)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
	})
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := ensureWallet(context.Background(), root)
			if !v.Ready || v.Available == nil || *v.Available != 12500 {
				t.Errorf("wallet not ready: %#v", v)
			}
		}()
	}
	wg.Wait()
	if binds.Load() != 1 {
		t.Fatalf("bound %d wallets", binds.Load())
	}
	b, _ := json.Marshal(ensureWallet(context.Background(), root))
	if strings.Contains(string(b), fixtureKey) {
		t.Fatal("wallet view exposed credential")
	}
}

func TestWalletRotationResumesWithoutMintingAgain(t *testing.T) {
	mints, commits := 0, 0
	fixtureCloud(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/device/rotate":
			mints++
			json.NewEncoder(w).Encode(map[string]string{"apiKey": fixtureKey + "-new"})
		case "/device/rotate/commit":
			commits++
			if commits == 1 {
				w.WriteHeader(503)
				return
			}
			w.Write([]byte(`{"balanceTokens":12500}`))
		case "/api/usage/token/":
			usageReply(w)
		}
	})
	root := t.TempDir()
	saveWallet(root, WalletState{APIKey: fixtureKey, WalletID: "same"})
	if _, err := walletAction(context.Background(), root, "wallet.rotate", map[string]any{"confirmed": true}); err == nil {
		t.Fatal("commit failure was hidden")
	}
	pending, _ := loadWallet(root)
	if pending.PendingKey == "" || pending.APIKey != fixtureKey {
		t.Fatal("pending or old key lost")
	}
	v := ensureWallet(context.Background(), root)
	state, _ := loadWallet(root)
	if !v.Ready || mints != 1 || commits != 2 || state.PendingKey != "" || state.APIKey != fixtureKey+"-new" || state.WalletID != "same" {
		t.Fatalf("failed recovery %#v / %d %d", state, mints, commits)
	}
}

func TestWalletFailurePreservesStateAndDamagedFileCanBeAdopted(t *testing.T) {
	fixtureCloud(t, func(w http.ResponseWriter, r *http.Request) { usageReply(w) })
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "data", "wallet"), 0700)
	p := filepath.Join(root, "data", "wallet", "device.json")
	old := []byte("{broken-wallet")
	os.WriteFile(p, old, 0600)
	if v := ensureWallet(context.Background(), root); v.Ready {
		t.Fatal("damaged wallet silently replaced")
	}
	b, _ := os.ReadFile(p)
	if string(b) != string(old) {
		t.Fatal("corrupted file lost")
	}
	if _, err := walletAction(context.Background(), root, "wallet.adopt", map[string]any{"api_key": fixtureKey}); err != nil {
		t.Fatal(err)
	}
	backups, _ := os.ReadDir(filepath.Join(root, "data", "backups"))
	if len(backups) == 0 {
		t.Fatal("damaged file not backed up")
	}
	state, _ := loadWallet(root)
	if state.APIKey != fixtureKey {
		t.Fatal("adopt failed")
	}
	saveWallet(root, WalletState{APIKey: fixtureKey, PendingKey: "unknown", PendingKind: "unrecognized"})
	v := ensureWallet(context.Background(), root)
	state, _ = loadWallet(root)
	if !v.Pending || state.PendingKey != "unknown" {
		t.Fatal("unknown pending silently removed")
	}
}

func TestCloudAgentReadsEvidenceButWritesWaitForUser(t *testing.T) {
	rounds := 0
	fixtureCloud(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/pricing" {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		if r.URL.Path == "/v1/models" {
			w.Write([]byte(`{"data":[{"id":"deepseek-v4-flash"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected route %s", r.URL.Path)
		}
		var request map[string]any
		json.NewDecoder(r.Body).Decode(&request)
		b, _ := json.Marshal(request)
		if strings.Contains(string(b), fixtureKey) {
			t.Fatal("wallet secret sent in model context")
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureKey {
			t.Fatal("missing transport authentication")
		}
		rounds++
		if rounds == 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"read","type":"function","function":{"name":"maintenance_action","arguments":"{\"action\":\"system.inspect\"}"}},{"id":"save","type":"function","function":{"name":"maintenance_action","arguments":"{\"action\":\"report.create\",\"confirmed\":true}"}},{"id":"evil","type":"function","function":{"name":"maintenance_action","arguments":"{\"action\":\"exec\",\"command\":\"delete-all\"}"}}]}}]}`))
			return
		}
		if !strings.Contains(string(b), "requires_user_confirmation") || !strings.Contains(string(b), "memory_bytes") || !strings.Contains(string(b), "未执行") {
			t.Fatal("model did not receive real evidence/confirmation refusal")
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"体检已完成，保存报告需要你确认。"}}]}`))
	})
	root := t.TempDir()
	saveWallet(root, WalletState{APIKey: fixtureKey})
	v, err := chatAction(context.Background(), root, "chat.start", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := v.(ChatSession)
	v, err = chatAction(context.Background(), root, "chat.send", map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "message": "检查电脑并保存报告", "engine": "cloud"})
	if err != nil {
		t.Fatal(err)
	}
	s = v.(ChatSession)
	if rounds != 2 || len(s.Pending) != 1 {
		t.Fatalf("agent did not form proposal: %#v", s)
	}
	reports, _ := listReports(root)
	if len(reports.([]any)) != 0 {
		t.Fatal("AI wrote without user approval")
	}
	input := map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "proposal_id": s.Pending[0].ID, "confirmed": true}
	if _, err = chatAction(context.Background(), root, "chat.confirm", input); err != nil {
		t.Fatal(err)
	}
	if _, err = chatAction(context.Background(), root, "chat.confirm", input); err == nil {
		t.Fatal("stale duplicate ran again")
	}
	reports, _ = listReports(root)
	if len(reports.([]any)) != 1 {
		t.Fatalf("unexpected reports %d", len(reports.([]any)))
	}
}

func TestWalletQueryFailureIsUnknownNotZero(t *testing.T) {
	fixtureCloud(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) })
	view := queryWallet(context.Background(), WalletState{APIKey: fixtureKey}, "")
	if !view.Ready || view.Available != nil {
		t.Fatal("failed query reported empty wallet")
	}
}

func TestSystemEvidenceDoesNotPresentAppVersionAsOSVersion(t *testing.T) {
	original := map[string]any{"os": "windows", "cpu_threads": 20, "version": "0.3.0-preview", "portable_root": "E:/private-root"}
	v := compactEvidence(map[string]any{"data": original}).(map[string]any)["data"].(map[string]any)
	if v["app_version"] != "0.3.0-preview" || v["version"] != nil || v["portable_root"] != nil {
		t.Fatalf("ambiguous or private system evidence: %#v", v)
	}
	if original["version"] != "0.3.0-preview" {
		t.Fatal("changed the legacy action response")
	}
}
