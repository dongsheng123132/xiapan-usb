package main

import (
	"testing"
)

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
