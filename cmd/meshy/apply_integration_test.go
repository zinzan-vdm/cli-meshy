package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// TestApplyRenewsAndRevokesWithTheRealTool proves the chain that apply builds
// with the pinned nebula-cert, not with a stub: the certificate in the vault is
// a real PEM, the blocklist holds the fingerprint that the tool prints, and a
// certificate inside the window is replaced. It runs only when MESHY_NEBULA_CERT
// names a binary, because the default run must not need a download.
func TestApplyRenewsAndRevokesWithTheRealTool(t *testing.T) {
	binary := os.Getenv("MESHY_NEBULA_CERT")
	if binary == "" {
		t.Skip("MESHY_NEBULA_CERT is not set")
	}

	t.Setenv("PATH", filepath.Dir(binary))
	dir := issueMeshDir(t, issueMesh)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	body, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 0 {
		t.Fatalf("ca init exit = %d: %s", code, body)
	}
	shares := sharesIn(t, body)
	if _, err := vault.Unlock(dir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if body, code := capture(t, func() int { return cmdIssue([]string{"--all", "--mesh", dir}) }); code != 0 {
		t.Fatalf("issue exit = %d: %s", code, body)
	}

	record := storedRecord(t, dir, shares)
	coreCert, _, found := vault.Certificate(record, "core-1")
	if !found {
		t.Fatal("the vault holds no certificate for core-1")
	}

	// A certificate that the file does not name is a node that left the mesh.
	vault.SetCertificate(record, "old-1", coreCert, []byte("old key\n"))
	if err := vault.Write(dir, filepath.Join(dir, vault.DirName), record); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	nodes := useFakes(t, "core-1", "web-1")
	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("apply exit = %d: %s", code, errOut)
	}
	if !strings.Contains(out, "revoked old-1 (") {
		t.Errorf("the report does not name the revocation: %q", out)
	}
	if !strings.Contains(out, "core-1: wrote blocklist.txt") {
		t.Errorf("the run did not take the blocklist to a node: %q", out)
	}

	printed, err := issue.Print(context.Background(), binary, coreCert)
	if err != nil {
		t.Fatalf("print the certificate: %v", err)
	}
	blocklist := nodes["core-1"].read("/etc/nebula/meshy-homelab/blocklist.txt")
	if strings.TrimSpace(blocklist) != printed.Fingerprint {
		t.Errorf("the blocklist holds %q, want the fingerprint %q", blocklist, printed.Fingerprint)
	}
	if got := nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt"); !strings.Contains(got, "BEGIN NEBULA CERTIFICATE") {
		t.Errorf("the node holds %q, want a certificate in the Nebula PEM form", got)
	}

	record, err = vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if _, _, found := vault.Certificate(record, "old-1"); found {
		t.Error("the vault still holds the revoked node")
	}

	// A second run against the same mesh changes nothing.
	out, errOut, code = captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("second apply exit = %d: %s", code, errOut)
	}
	if strings.Contains(out, "wrote") {
		t.Errorf("the second run wrote to a node: %q", out)
	}

	// A certificate inside the window is replaced on the next run.
	authorityCert, authorityKey, found := vault.Certificate(record, "ca-1")
	if !found {
		t.Fatal("the vault holds no authority")
	}
	short, err := issue.Sign(context.Background(), issue.Options{
		Binary:    binary,
		Authority: issue.Authority{Cert: authorityCert, Key: authorityKey},
		Request: issue.Request{
			Name:     "core-1",
			Networks: "10.42.0.1/24",
			Groups:   []string{"core", "edge"},
			Lifetime: time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("sign a certificate that is inside the window: %v", err)
	}
	vault.SetCertificate(record, "core-1", short.Cert, short.Key)
	if err := vault.Write(dir, filepath.Join(dir, vault.DirName), record); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	out, errOut, code = captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 0 {
		t.Fatalf("renewal exit = %d: %s", code, errOut)
	}
	if !strings.Contains(out, "core-1: renewed, wrote") {
		t.Errorf("the report does not name the renewal: %q", out)
	}
}
