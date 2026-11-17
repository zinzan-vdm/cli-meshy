package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// TestIssueSignsWithTheRealTool proves the chain that issue writes with the
// pinned nebula-cert, not with a stub. It runs only when MESHY_NEBULA_CERT names
// a binary, because the pattern keeps the default test run free of downloads.
func TestIssueSignsWithTheRealTool(t *testing.T) {
	binary := os.Getenv("MESHY_NEBULA_CERT")
	if binary == "" {
		t.Skip("MESHY_NEBULA_CERT is not set")
	}

	t.Setenv("PATH", filepath.Dir(binary))
	dir := issueMeshDir(t, issueMesh)

	body, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 0 {
		t.Fatalf("ca init exit = %d: %s", code, body)
	}
	shares := sharesIn(t, body)
	if _, err := vault.Unlock(dir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	body, code = capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("issue exit = %d: %s", code, body)
	}
	if !strings.Contains(body, "issued") {
		t.Errorf("output does not report a certificate: %s", body)
	}

	record := storedRecord(t, dir, shares)
	cert, key, found := vault.Certificate(record, "core-1")
	if !found || len(key) == 0 {
		t.Fatalf("the vault holds no certificate and key for core-1: %v", record)
	}

	work := t.TempDir()
	certPath := filepath.Join(work, "core-1.crt")
	caPath := filepath.Join(work, "ca-1.crt")
	for path, content := range map[string][]byte{certPath: cert, caPath: record["ca-1.crt"]} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	var node []struct {
		Details struct {
			Name     string   `json:"name"`
			Issuer   string   `json:"issuer"`
			Groups   []string `json:"groups"`
			Networks []string `json:"networks"`
		} `json:"details"`
	}
	printed, err := exec.Command(binary, "print", "-json", "-path", certPath).Output()
	if err != nil {
		t.Fatalf("print the node certificate: %v", err)
	}
	if err := json.Unmarshal(printed, &node); err != nil {
		t.Fatalf("decode the node certificate: %v", err)
	}
	if len(node) != 1 {
		t.Fatalf("the tool printed %d certificates, want 1", len(node))
	}

	var authority []struct {
		Fingerprint string `json:"fingerprint"`
	}
	printed, err = exec.Command(binary, "print", "-json", "-path", caPath).Output()
	if err != nil {
		t.Fatalf("print the authority certificate: %v", err)
	}
	if err := json.Unmarshal(printed, &authority); err != nil {
		t.Fatalf("decode the authority certificate: %v", err)
	}
	if len(authority) != 1 {
		t.Fatalf("the tool printed %d authorities, want 1", len(authority))
	}

	if node[0].Details.Name != "core-1" {
		t.Errorf("name = %q", node[0].Details.Name)
	}
	if got := strings.Join(node[0].Details.Groups, ","); got != "core,edge" {
		t.Errorf("groups = %q, want core,edge", got)
	}
	if len(node[0].Details.Networks) != 1 || node[0].Details.Networks[0] != "10.42.0.1/24" {
		t.Errorf("networks = %v, want [10.42.0.1/24]", node[0].Details.Networks)
	}
	if !strings.EqualFold(node[0].Details.Issuer, authority[0].Fingerprint) {
		t.Errorf("issuer = %q, want the authority fingerprint %q", node[0].Details.Issuer, authority[0].Fingerprint)
	}

	before := hashFile(t, filepath.Join(dir, vault.FileName))
	body, code = capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("second issue exit = %d: %s", code, body)
	}
	if !strings.Contains(body, "already valid until") {
		t.Errorf("the second run does not report a valid certificate: %s", body)
	}
	if after := hashFile(t, filepath.Join(dir, vault.FileName)); after != before {
		t.Error("the second run changed the sealed vault")
	}
}
