package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovedFeaturesCannotRunThroughCoreOrAgent(t *testing.T) {
	root := t.TempDir()
	removed := []string{"wallet.ensure", "wallet.refresh", "wallet.adopt", "wallet.copy", "wallet.rotate", "wallet.reset", "wallet.recharge", "cloud.models", "models.start", "models.stop", "models.inspect", "tools.install", "resources.scan", "maintenance.plan", "network.diagnose"}
	schema, _ := json.Marshal(maintenanceToolSchema())
	for _, id := range removed {
		for _, a := range actions {
			if a.ID == id {
				t.Fatalf("removed action registered: %s", id)
			}
		}
		if strings.Contains(string(schema), `"`+id+`"`) {
			t.Fatalf("removed action advertised to AI: %s", id)
		}
		if r := runAction(context.Background(), root, id, map[string]any{"confirmed": true, "api_key": "fixture", "package_id": "portable-check"}); r.OK {
			t.Fatalf("removed action ran: %s", id)
		}
		session := ChatSession{ID: uniqueID()}
		output := modelTool(context.Background(), root, &session, id, map[string]any{"confirmed": true})
		b, _ := json.Marshal(output)
		if !strings.Contains(string(b), `"ok":false`) || len(session.Pending) != 0 {
			t.Fatalf("removed action accepted by AI: %s", id)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("removed action changed user state: %v %v", entries, err)
	}
}

func TestLegacyCloudSettingsStayUntouchedUntilCompanyAPISaved(t *testing.T) {
	root := t.TempDir()
	original := []byte(`{"revision":"legacy","source":"cloud","model":"old-model"}`)
	path, err := newFileAtomic(root, []string{"data", "settings", "model.json"}, original)
	if err != nil {
		t.Fatal(err)
	}
	wallet := []byte(`{"apiKey":"legacy-wallet-fixture"}`)
	walletPath, err := newFileAtomic(root, []string{"data", "wallet", "device.json"}, wallet)
	if err != nil {
		t.Fatal(err)
	}
	s, err := selectedModel(root)
	if err != nil || s.Source != "custom" || s.Model != "" || s.BaseURL != "" || s.APIKey != "" {
		t.Fatalf("legacy cloud was not inert: %#v %v", s, err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != string(original) {
		t.Fatal("read rewrote legacy settings")
	}
	_, err = modelSettingsAction(context.Background(), root, "settings.model.save", map[string]any{"expected_revision": "legacy", "source": "custom", "model": "company-model", "base_url": "https://api.example.org/v1"})
	if err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(root, "data", "backups", "*model.json"))
	if len(backups) != 1 {
		t.Fatal("legacy configuration not backed up")
	}
	b, _ = os.ReadFile(backups[0])
	if string(b) != string(original) {
		t.Fatal("backup differs from original")
	}
	b, _ = os.ReadFile(walletPath)
	if string(b) != string(wallet) {
		t.Fatal("legacy wallet changed")
	}
}

func TestOfflineInspectionAndConfirmedReportPreserveEvidence(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	v, err := chatAction(ctx, root, "chat.start", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := v.(ChatSession)
	send := func(message string) {
		t.Helper()
		v, err = chatAction(ctx, root, "chat.send", map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "message": message})
		if err != nil {
			t.Fatal(err)
		}
		s = v.(ChatSession)
	}
	send("/体检")
	if len(s.Messages) != 2 || s.Messages[1].Action != "system.inspect" {
		t.Fatalf("inspection needed an AI configuration: %#v", s.Messages)
	}
	send("/报告")
	if len(s.Pending) != 1 {
		t.Fatalf("report confirmation missing: %#v", s.Pending)
	}
	if _, err = os.Stat(filepath.Join(root, "data", "reports")); !os.IsNotExist(err) {
		t.Fatal("report created before confirmation")
	}
	input := map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "proposal_id": s.Pending[0].ID, "confirmed": true}
	v, err = chatAction(ctx, root, "chat.confirm", input)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := listReports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.([]any)) != 1 {
		t.Fatalf("expected one report: %#v", rows)
	}
	report := rows.([]any)[0].(map[string]any)
	evidence := report["evidence"].([]any)
	if len(evidence) != 1 || evidence[0].(map[string]any)["action"] != "system.inspect" {
		t.Fatalf("inspection evidence missing: %#v", report)
	}
	if _, err = chatAction(ctx, root, "chat.confirm", input); err == nil {
		t.Fatal("stale confirmation accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "data", "wallet")); !os.IsNotExist(err) {
		t.Fatal("offline flow created a wallet")
	}
}

func TestEmbeddedNetworkEngineContainsNoRepairImplementations(t *testing.T) {
	text := strings.ToLower(string(networkEngineScript))
	for _, command := range []string{"function repair-", "function clear-proxy", "function set-dns", "function require-admin", "set-itemproperty", "remove-itemproperty", "netsh winsock reset", "ipconfig /release"} {
		if strings.Contains(text, command) {
			t.Fatalf("removed mutation survived in engine: %s", command)
		}
	}
}
