package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// TestCARotatePhase2SignsWithTheRealTool proves the chain of the rotation with
// the pinned nebula-cert. It runs only when MESHY_NEBULA_CERT names a binary,
// because the pattern keeps the default test run free of downloads.
func TestCARotatePhase2SignsWithTheRealTool(t *testing.T) {
	binary := os.Getenv("MESHY_NEBULA_CERT")
	if binary == "" {
		t.Skip("MESHY_NEBULA_CERT is not set")
	}
	t.Setenv("PATH", filepath.Dir(binary))

	dir := writeApplyMesh(t)
	initOut, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 0 {
		t.Fatalf("ca init exit = %d: %s", code, initOut)
	}
	shares := sharesIn(t, initOut)
	if len(shares) < vault.Threshold {
		t.Fatalf("ca init printed %d shares, want %d", len(shares), vault.ShareCount)
	}
	if _, err := vault.Unlock(dir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock the vault: %v", err)
	}

	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1")
	rotateNode(nodes, "web-1", "ca-1")

	if body, code := capture(t, func() int { return run([]string{"apply", "--mesh", dir}) }); code != 0 {
		t.Fatalf("apply exit = %d, want 0: %s", code, body)
	}
	oldCore := nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt")
	if !strings.Contains(oldCore, "BEGIN NEBULA CERTIFICATE") {
		t.Fatalf("the node holds no certificate: %q", oldCore)
	}

	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	if body, code := capture(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	}); code != 0 {
		t.Fatalf("phase 1 exit = %d, want 0: %s", code, body)
	}

	// The issuer of the new certificate is the fingerprint of ca-2, so the test
	// reads it from the tool before the phase runs.
	record := readVault(t, dir)
	caOne := writeTemp(t, "ca-1.crt", record["ca-1.crt"])
	caTwo := writeTemp(t, "ca-2.crt", record["ca-2.crt"])
	first := fingerprintOf(t, binary, caOne)
	second := fingerprintOf(t, binary, caTwo)
	rotateCert(nodes, "core-1", first)
	rotateCert(nodes, "web-1", first)
	rotateCertAfter(nodes, "core-1", second)
	rotateCertAfter(nodes, "web-1", second)

	body, code := capture(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("phase 2 exit = %d, want 0: %s", code, body)
	}
	if !strings.Contains(body, "next: meshy ca rotate --phase 3") {
		t.Errorf("the output does not name the next phase: %s", body)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-2" {
		t.Errorf("active = %q, want ca-2", mesh.CA.Active)
	}

	record = readVault(t, dir)
	fresh := writeTemp(t, "core-1.crt", record["core-1.crt"])
	previous, _, found := vault.Previous(record, "core-1", "ca-1")
	if !found {
		t.Fatal("the vault dropped the certificate that ca-1 signed")
	}
	kept := writeTemp(t, "core-1-old.crt", previous)

	// The new certificate verifies against ca-2 and not against ca-1. The old
	// certificate still verifies against ca-1, so it stays valid until phase 3.
	if out, err := exec.Command(binary, "verify", "-ca", caTwo, "-crt", fresh).CombinedOutput(); err != nil {
		t.Errorf("the new certificate does not verify against ca-2: %v: %s", err, out)
	}
	if out, err := exec.Command(binary, "verify", "-ca", caOne, "-crt", fresh).CombinedOutput(); err == nil {
		t.Errorf("the new certificate still verifies against ca-1: %s", out)
	}
	if out, err := exec.Command(binary, "verify", "-ca", caOne, "-crt", kept).CombinedOutput(); err != nil {
		t.Errorf("the old certificate no longer verifies against ca-1: %v: %s", err, out)
	}
	if string(previous) != oldCore {
		t.Error("the pair that the vault kept is not the certificate that the node held")
	}
}

func readVault(t *testing.T, dir string) vault.Record {
	t.Helper()
	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	return record
}

func writeTemp(t *testing.T, name string, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func fingerprintOf(t *testing.T, binary, path string) string {
	t.Helper()
	out, err := exec.Command(binary, "print", "-json", "-path", path).Output()
	if err != nil {
		t.Fatalf("print %s: %v", path, err)
	}
	cert, err := node.ParseCert(string(out))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return cert.Fingerprint
}
