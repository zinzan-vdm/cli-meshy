package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

const caMesh = `schema: 1
name: homelab
cidr: 10.42.0.0/24
interface: meshy0
nebula:
  version: v1.12.0
ca:
  duration: 20y
  rotate_at: 80%
  trust: [ca-1]
  active: ca-1
lighthouses:
  - node: core-1
    port: 4242
nodes:
  - name: core-1
    kind: server
    groups: [core]
    ip: 10.42.0.1
    ssh:
      user: meshy
      host: 203.0.113.10
    public_addr: 203.0.113.10:4242
rules: []
`

// stubBinary writes a nebula-cert that records its arguments and writes the two
// files the command asks for. The test reads the arguments to prove what the
// command asked of the tool.
func stubBinary(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	args := filepath.Join(t.TempDir(), "args")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$STUB_ARGS"
key=""
crt=""
while [ $# -gt 0 ]; do
  case "$1" in
    -out-key) key="$2"; shift 2 ;;
    -out-crt) crt="$2"; shift 2 ;;
    *) shift ;;
  esac
done
printf 'STUB KEY\n' > "$key"
printf 'STUB CRT\n' > "$crt"
`
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	t.Setenv("PATH", dir)
	t.Setenv("STUB_ARGS", args)
}

func writeCAMesh(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

func captureErr(t *testing.T, fn func() int) (string, int) {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	code := fn()

	w.Close()
	os.Stderr = old
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(body), code
}

func sharesIn(t *testing.T, body string) []string {
	t.Helper()

	var shares []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "meshy-") {
			shares = append(shares, line)
		}
	}
	return shares
}

func hashFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestCAInitSealsTheAuthority(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, caMesh)

	body, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}

	shares := sharesIn(t, body)
	if len(shares) != vault.ShareCount {
		t.Fatalf("printed %d shares, want %d:\n%s", len(shares), vault.ShareCount, body)
	}

	sealed := filepath.Join(dir, vault.FileName)
	info, err := os.Stat(sealed)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}

	ciphertext, err := os.ReadFile(sealed)
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

	if string(record["ca-1.key"]) != "STUB KEY\n" {
		t.Errorf("key = %q", record["ca-1.key"])
	}
	if string(record["ca-1.crt"]) != "STUB CRT\n" {
		t.Errorf("crt = %q", record["ca-1.crt"])
	}
	if _, ok := record["ca-1.issued"]; !ok {
		t.Error("the record holds no issue time")
	}
}

func TestCAInitAsksForTheLifetimeAndTheRange(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, caMesh)

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) }); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	body, err := os.ReadFile(os.Getenv("STUB_ARGS"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(body)

	for _, want := range []string{"ca\n", "-name\nca-1\n", "-duration\n175200h\n", "-networks\n10.42.0.0/24\n"} {
		if !strings.Contains(args, want) {
			t.Errorf("arguments do not hold %q:\n%s", want, args)
		}
	}
}

func TestCAInitRefusesASecondRun(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, caMesh)

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) }); code != 0 {
		t.Fatalf("first run exit = %d", code)
	}
	before := hashFile(t, filepath.Join(dir, vault.FileName))

	stderr, code := captureErr(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 1 {
		t.Errorf("second run exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("message does not name --force: %s", stderr)
	}
	if after := hashFile(t, filepath.Join(dir, vault.FileName)); after != before {
		t.Error("the second run changed the vault")
	}
}

func TestCAInitForceReplacesTheVault(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, caMesh)

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) }); code != 0 {
		t.Fatalf("first run exit = %d", code)
	}
	before := hashFile(t, filepath.Join(dir, vault.FileName))

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--force", "--mesh", dir}) }); code != 0 {
		t.Fatalf("second run exit = %d, want 0", code)
	}
	if after := hashFile(t, filepath.Join(dir, vault.FileName)); after == before {
		t.Error("--force did not replace the vault")
	}
}

func TestCAInitRefusesAnInvalidMesh(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, strings.Replace(caMesh, "ip: 10.42.0.1", "ip: 10.99.0.1", 1))

	stderr, code := captureErr(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "outside cidr") {
		t.Errorf("message does not name the problem: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, vault.FileName)); err == nil {
		t.Error("an invalid mesh produced a vault")
	}
}

func TestCAInitFailsWhenThereIsNoBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := writeCAMesh(t, caMesh)

	stderr, code := captureErr(t, func() int { return cmdCAInit([]string{"--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "nebula-cert") {
		t.Errorf("message does not name the binary: %s", stderr)
	}
}

func TestCAInitAddsTheAuthorityToTheMeshFile(t *testing.T) {
	stubBinary(t)
	withoutCA := strings.Replace(caMesh, `ca:
  duration: 20y
  rotate_at: 80%
  trust: [ca-1]
  active: ca-1
`, "", 1)
	dir := writeCAMesh(t, withoutCA)

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) }); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-1" || len(mesh.CA.Trust) != 1 || mesh.CA.Trust[0] != "ca-1" {
		t.Errorf("ca block = %+v, want trust [ca-1] and active ca-1", mesh.CA)
	}
}

func TestCAInitLeavesACompleteMeshFileAlone(t *testing.T) {
	stubBinary(t)
	dir := writeCAMesh(t, caMesh)
	before := hashFile(t, filepath.Join(dir, config.FileName))

	if _, code := capture(t, func() int { return cmdCAInit([]string{"--mesh", dir}) }); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	if after := hashFile(t, filepath.Join(dir, config.FileName)); after != before {
		t.Error("a complete mesh file was rewritten")
	}
}
