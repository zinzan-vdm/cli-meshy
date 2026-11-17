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

// TestCAInitSignsWithTheRealTool proves the certificate chain with the pinned
// nebula-cert, not with a stub. It runs only when MESHY_NEBULA_CERT names a
// binary, because the pattern keeps the default test run free of downloads.
func TestCAInitSignsWithTheRealTool(t *testing.T) {
	binary := os.Getenv("MESHY_NEBULA_CERT")
	if binary == "" {
		t.Skip("MESHY_NEBULA_CERT is not set")
	}

	t.Setenv("PATH", filepath.Dir(binary))
	dir := writeCAMesh(t, caMesh)

	body, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}

	shares := sharesIn(t, body)
	ciphertext, err := os.ReadFile(filepath.Join(dir, vault.FileName))
	if err != nil {
		t.Fatalf("read vault: %v", err)
	}
	plaintext, err := vault.Open(ciphertext, shares[:vault.Threshold])
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	record, err := vault.Decode(plaintext)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	work := t.TempDir()
	caCrt := filepath.Join(work, "ca-1.crt")
	caKey := filepath.Join(work, "ca-1.key")
	hostCrt := filepath.Join(work, "core-1.crt")
	hostKey := filepath.Join(work, "core-1.key")
	for path, content := range map[string][]byte{caCrt: record["ca-1.crt"], caKey: record["ca-1.key"]} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	sign := exec.Command(binary, "sign",
		"-ca-crt", caCrt, "-ca-key", caKey,
		"-name", "core-1", "-networks", "10.42.0.1/24",
		"-out-crt", hostCrt, "-out-key", hostKey,
	)
	if out, err := sign.CombinedOutput(); err != nil {
		t.Fatalf("sign: %v: %s", err, out)
	}

	printed, err := exec.Command(binary, "print", "-json", "-path", caCrt).Output()
	if err != nil {
		t.Fatalf("print ca: %v", err)
	}
	var authority []struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(printed, &authority); err != nil {
		t.Fatalf("decode ca: %v", err)
	}
	if len(authority) != 1 {
		t.Fatalf("the tool printed %d certificates, want 1", len(authority))
	}

	printed, err = exec.Command(binary, "print", "-json", "-path", hostCrt).Output()
	if err != nil {
		t.Fatalf("print host: %v", err)
	}
	var host []struct {
		Details struct {
			Name     string   `json:"name"`
			Issuer   string   `json:"issuer"`
			Networks []string `json:"networks"`
		} `json:"details"`
	}
	if err := json.Unmarshal(printed, &host); err != nil {
		t.Fatalf("decode host: %v", err)
	}
	if len(host) != 1 {
		t.Fatalf("the tool printed %d certificates, want 1", len(host))
	}

	if host[0].Details.Name != "core-1" {
		t.Errorf("host name = %q", host[0].Details.Name)
	}
	if !strings.EqualFold(host[0].Details.Issuer, authority[0].Fingerprint) {
		t.Errorf("issuer = %q, want the authority fingerprint %q", host[0].Details.Issuer, authority[0].Fingerprint)
	}
	if len(host[0].Details.Networks) != 1 || host[0].Details.Networks[0] != "10.42.0.1/24" {
		t.Errorf("networks = %v, want [10.42.0.1/24]", host[0].Details.Networks)
	}
}
