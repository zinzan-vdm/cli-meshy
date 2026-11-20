package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

const issueMesh = `schema: 1
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
certificates:
  server: 1y
  user: 90d
  renew_before: 60d
lighthouses:
  - node: core-1
    port: 4242
nodes:
  - name: core-1
    kind: server
    groups: [core, edge]
    ip: 10.42.0.1
    ssh:
      user: meshy
      host: 203.0.113.10
    public_addr: 203.0.113.10:4242
  - name: web-1
    kind: server
    groups: [web]
    ip: 10.42.0.2
    ssh:
      user: meshy
      host: 203.0.113.11
  - name: laptop-1
    kind: user
    groups: [people]
    ip: 10.42.0.50
    platform: darwin/arm64
rules: []
`

// stubSigner writes a nebula-cert that records its arguments, writes the two
// files a sign asks for, and answers a print with the expiry of the test.
func stubSigner(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "print" ]; then
  printf '[{"details":{"name":"stub","groups":[],"networks":[],"notAfter":"%s","issuer":"%s"},"fingerprint":"%s"}]\n' "$STUB_EXPIRY" "$STUB_ISSUER" "$STUB_FINGERPRINT"
  exit 0
fi
printf '%s\n' "$@" > "$STUB_ARGS"
n=0
if [ -f "$STUB_COUNT" ]; then read n < "$STUB_COUNT"; fi
n=$((n + 1))
printf '%s' "$n" > "$STUB_COUNT"
while [ $# -gt 0 ]; do
  case "$1" in
    -out-key) printf 'STUB KEY %s\n' "$n" > "$2"; shift 2 ;;
    -out-crt) printf 'STUB CRT %s\n' "$n" > "$2"; shift 2 ;;
    *) shift ;;
  esac
done
`
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	t.Setenv("PATH", dir)
	t.Setenv("STUB_ARGS", filepath.Join(t.TempDir(), "args"))
	t.Setenv("STUB_COUNT", filepath.Join(t.TempDir(), "count"))
	t.Setenv("STUB_FINGERPRINT", "4f3a")
	// A print names the issuer, because a rotation reads it to name the
	// authority that signed a certificate.
	t.Setenv("STUB_ISSUER", "4f3a")
	setExpiry(t, time.Now().Add(300*24*time.Hour))
}

// setExpiry makes the stub print one expiry and returns its date form.
func setExpiry(t *testing.T, when time.Time) string {
	t.Helper()

	at := when.UTC().Truncate(time.Second)
	t.Setenv("STUB_EXPIRY", at.Format(time.RFC3339))
	return at.Format(issueDateFormat)
}

func issueMeshDir(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

// openIssueVault writes the vault that `meshy ca init` writes and opens it.
func openIssueVault(t *testing.T, dir string) []string {
	t.Helper()

	plaintext, err := vault.Encode(vault.Record{
		"ca-1.key":    []byte("CA KEY"),
		"ca-1.crt":    []byte("CA CRT"),
		"ca-1.issued": []byte("2026-11-17T20:00:00Z"),
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	ciphertext, shares, err := vault.Seal(plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, vault.FileName), ciphertext, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}
	if _, err := vault.Unlock(dir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	return shares
}

func storedRecord(t *testing.T, dir string, shares []string) vault.Record {
	t.Helper()

	ciphertext, err := os.ReadFile(filepath.Join(dir, vault.FileName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	plaintext, err := vault.Open(ciphertext, shares[:vault.Threshold])
	if err != nil {
		t.Fatalf("open the vault: %v", err)
	}
	record, err := vault.Decode(plaintext)
	if err != nil {
		t.Fatalf("decode the vault: %v", err)
	}
	return record
}

func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()

	sums := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		sums[entry.Name()] = hashFile(t, filepath.Join(dir, entry.Name()))
	}
	return sums
}

func TestIssueSignsANewCertificate(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	shares := openIssueVault(t, dir)

	body, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}
	if !strings.Contains(body, "issued") || !strings.Contains(body, "4f3a") {
		t.Errorf("output does not report the certificate: %s", body)
	}

	for name, want := range map[string]string{"core-1.crt": "STUB CRT 1\n", "core-1.key": "STUB KEY 1\n"} {
		got, err := os.ReadFile(filepath.Join(dir, vault.DirName, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	record := storedRecord(t, dir, shares)
	cert, key, found := vault.Certificate(record, "core-1")
	if !found {
		t.Fatalf("the sealed vault holds no certificate for core-1: %v", record)
	}
	if string(cert) != "STUB CRT 1\n" || string(key) != "STUB KEY 1\n" {
		t.Errorf("the sealed vault holds %q and %q", cert, key)
	}
	if string(record["ca-1.key"]) != "CA KEY" {
		t.Errorf("the authority key left the vault: %q", record["ca-1.key"])
	}
}

func TestIssueAsksForTheNodeFacts(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	if _, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) }); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	body, err := os.ReadFile(os.Getenv("STUB_ARGS"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(body)

	for _, want := range []string{
		"sign\n",
		"-name\ncore-1\n",
		"-networks\n10.42.0.1/24\n",
		"-groups\ncore,edge\n",
		"-duration\n8760h\n",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("arguments do not hold %q:\n%s", want, args)
		}
	}
}

func TestIssueUsesTheLifetimeOfTheKind(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	if _, code := capture(t, func() int { return cmdIssue([]string{"laptop-1", "--mesh", dir}) }); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	body, err := os.ReadFile(os.Getenv("STUB_ARGS"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(body)

	for _, want := range []string{"-duration\n2160h\n", "-networks\n10.42.0.50/24\n"} {
		if !strings.Contains(args, want) {
			t.Errorf("arguments do not hold %q:\n%s", want, args)
		}
	}
}

func TestIssueTwiceChangesNothing(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	if _, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) }); code != 0 {
		t.Fatalf("first run exit = %d", code)
	}
	before := hashTree(t, filepath.Join(dir, vault.DirName))
	sealedBefore := hashFile(t, filepath.Join(dir, vault.FileName))
	until := setExpiry(t, time.Now().Add(300*24*time.Hour))

	body, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("second run exit = %d, want 0: %s", code, body)
	}
	if !strings.Contains(body, "already valid until "+until) {
		t.Errorf("output does not report the valid certificate: %s", body)
	}

	if after := hashTree(t, filepath.Join(dir, vault.DirName)); !sameSums(before, after) {
		t.Error("the second run changed the open vault")
	}
	if after := hashFile(t, filepath.Join(dir, vault.FileName)); after != sealedBefore {
		t.Error("the second run changed the sealed vault")
	}
}

func TestIssueReplacesAnExpiringCertificate(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	if _, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) }); code != 0 {
		t.Fatalf("first run exit = %d", code)
	}

	setExpiry(t, time.Now().Add(10*24*time.Hour))
	body, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("second run exit = %d, want 0: %s", code, body)
	}
	if !strings.Contains(body, "issued") {
		t.Errorf("output does not report a new certificate: %s", body)
	}

	got, err := os.ReadFile(filepath.Join(dir, vault.DirName, "core-1.crt"))
	if err != nil {
		t.Fatalf("read the certificate: %v", err)
	}
	if string(got) != "STUB CRT 2\n" {
		t.Errorf("certificate = %q, want the one from the second run", got)
	}
}

func TestIssueForceSignsAgain(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	if _, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) }); code != 0 {
		t.Fatalf("first run exit = %d", code)
	}
	if _, code := capture(t, func() int { return cmdIssue([]string{"core-1", "--force", "--mesh", dir}) }); code != 0 {
		t.Fatalf("forced run exit = %d", code)
	}

	got, err := os.ReadFile(filepath.Join(dir, vault.DirName, "core-1.crt"))
	if err != nil {
		t.Fatalf("read the certificate: %v", err)
	}
	if string(got) != "STUB CRT 2\n" {
		t.Errorf("certificate = %q, want a new one", got)
	}
}

func TestIssueAllSignsEveryNode(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	body, code := capture(t, func() int { return cmdIssue([]string{"--all", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}
	if got := strings.Count(body, "issued"); got != 3 {
		t.Errorf("output reports %d certificates, want 3: %s", got, body)
	}

	for _, name := range []string{"core-1", "web-1", "laptop-1"} {
		if _, err := os.Stat(filepath.Join(dir, vault.DirName, name+".crt")); err != nil {
			t.Errorf("%s has no certificate: %v", name, err)
		}
	}
}

func TestIssueRefusesAnUnknownNode(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	stderr, code := captureErr(t, func() int { return cmdIssue([]string{"nope", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, `node "nope" is not in mesh.yml`) {
		t.Errorf("message does not name the node: %s", stderr)
	}
}

func TestIssueRefusesALockedVault(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)

	stderr, code := captureErr(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "the vault is locked, run meshy unlock") {
		t.Errorf("message does not say the vault is locked: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, vault.DirName, "core-1.crt")); err == nil {
		t.Error("a locked vault produced a certificate")
	}
}

func TestIssueRefusesAnInvalidMesh(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, strings.Replace(issueMesh, "ip: 10.42.0.2", "ip: 10.99.0.2", 1))
	openIssueVault(t, dir)

	stderr, code := captureErr(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "outside cidr") {
		t.Errorf("message does not name the problem: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, vault.DirName, "core-1.crt")); err == nil {
		t.Error("an invalid mesh produced a certificate")
	}
}

func TestIssueNeedsANodeOrAll(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	for name, args := range map[string][]string{
		"no node":        {"--mesh", dir},
		"a node and all": {"core-1", "--all", "--mesh", dir},
	} {
		stderr, code := captureErr(t, func() int { return cmdIssue(args) })
		if code != 2 {
			t.Errorf("%s: exit = %d, want 2", name, code)
		}
		if !strings.Contains(stderr, "--all") {
			t.Errorf("%s: message does not name the flag: %s", name, stderr)
		}
	}
}

func TestIssueRefusesASecondName(t *testing.T) {
	stubSigner(t)
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	stderr, code := captureErr(t, func() int { return cmdIssue([]string{"core-1", "web-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, `unexpected argument "web-1"`) {
		t.Errorf("message does not name the argument: %s", stderr)
	}
}

func TestIssueFailsWhenThereIsNoBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := issueMeshDir(t, issueMesh)
	openIssueVault(t, dir)

	stderr, code := captureErr(t, func() int { return cmdIssue([]string{"core-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "nebula-cert") {
		t.Errorf("message does not name the binary: %s", stderr)
	}
}

func sameSums(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for name, sum := range before {
		if after[name] != sum {
			return false
		}
	}
	return true
}
