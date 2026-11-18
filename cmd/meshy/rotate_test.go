package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// authorityPrint is the answer that `nebula-cert print -json` gives for a trust
// bundle that holds names. A real print has more fields, and the command reads
// the name and the fingerprint only.
func authorityPrint(names ...string) string {
	docs := make([]string, 0, len(names))
	for _, name := range names {
		docs = append(docs, `{"details":{"name":"`+name+`","isCa":true},"fingerprint":"ff`+name+`"}`)
	}
	return "[" + strings.Join(docs, ",") + "]"
}

// rotateNode sets the answer that the node gives for its trust bundle, so the
// read back compares the list that the test names.
func rotateNode(nodes map[string]*fakeNode, name string, trust ...string) {
	nodes[name].prints["/etc/nebula/meshy-homelab/ca.crt"] = authorityPrint(trust...)
}

func TestCARotatePhase1CreatesTheNextAuthority(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	for _, want := range []string{
		"authority ca-2 is sealed in " + vault.FileName,
		"mesh.yml now trusts ca-1 and ca-2",
		"next: meshy ca rotate --phase 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output does not hold %q:\n%s", want, out)
		}
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if _, _, found := vault.Certificate(record, "ca-2"); !found {
		t.Error("the vault holds no ca-2")
	}
	if _, ok := record["ca-2.issued"]; !ok {
		t.Error("the vault holds no issue time for ca-2")
	}
	if _, _, found := vault.Certificate(record, "ca-1"); !found {
		t.Error("the run removed ca-1 from the vault")
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 2 || mesh.CA.Trust[0] != "ca-1" || mesh.CA.Trust[1] != "ca-2" {
		t.Errorf("trust = %v, want [ca-1 ca-2]", mesh.CA.Trust)
	}
	if mesh.CA.Active != "ca-1" {
		t.Errorf("active = %q, want ca-1", mesh.CA.Active)
	}

	for _, name := range []string{"core-1", "web-1"} {
		bundle := nodes[name].read("/etc/nebula/meshy-homelab/ca.crt")
		if !strings.Contains(bundle, "authority certificate") || !strings.Contains(bundle, "STUB CRT") {
			t.Errorf("%s holds the bundle %q", name, bundle)
		}
		if nodes[name].reloads != 1 {
			t.Errorf("%s reloaded %d times, want 1", name, nodes[name].reloads)
		}
	}
}

func TestCARotatePhase1IssuesNoCertificate(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}

	body, err := os.ReadFile(os.Getenv("STUB_ARGS"))
	if err != nil {
		t.Fatalf("read the arguments: %v", err)
	}
	args := string(body)
	if !strings.HasPrefix(args, "ca\n") {
		t.Errorf("the tool was not asked for an authority: %s", args)
	}
	if strings.Contains(args, "sign") {
		t.Errorf("the tool was asked to sign: %s", args)
	}
	if !strings.Contains(args, "-name\nca-2\n") {
		t.Errorf("the tool was not asked for ca-2: %s", args)
	}
	if got := nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt"); got != "core-1 certificate\n" {
		t.Errorf("the node certificate is %q, want the certificate from the vault", got)
	}
}

func TestCARotatePhase1RefusesATrustOfTwo(t *testing.T) {
	dir := writeApplyMesh(t)
	path := filepath.Join(dir, config.FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	broken := strings.Replace(string(body), "trust: [ca-1]", "trust: [ca-1, ca-2]", 1)
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "ca.trust already holds two authorities") {
		t.Errorf("message = %q, want the refusal", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, vault.FileName)); err == nil {
		t.Error("a refused run sealed a vault")
	}
}

func TestCARotatePhase1SaysUnlockWhenTheVaultIsClosed(t *testing.T) {
	dir := writeApplyMesh(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "the vault is locked, run meshy unlock") {
		t.Errorf("message = %q, want the unlock line", stderr)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 1 {
		t.Errorf("a refused run changed ca.trust: %v", mesh.CA.Trust)
	}
}

func TestCARotatePhase1ContinuesAfterANodeFails(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	nodes["core-1"].failAll = true
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "core-1: failed: ") {
		t.Errorf("the report does not name the failed node:\n%s", out)
	}
	if !strings.Contains(out, "web-1: wrote") {
		t.Errorf("the run stopped before the second node:\n%s", out)
	}
	if nodes["web-1"].reloads != 1 {
		t.Errorf("web-1 reloaded %d times, want 1", nodes["web-1"].reloads)
	}
}

func TestCARotatePhase1ReportsANodeThatDidNotTakeTheBundle(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")
	rotateNode(nodes, "core-1", "ca-1")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "does not hold ca-2") {
		t.Errorf("the report does not name the missing authority:\n%s", out)
	}
}

func TestCARotateNeedsAPhase(t *testing.T) {
	dir := writeApplyMesh(t)

	stderr, code := captureErr(t, func() int { return run([]string{"ca", "rotate", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--phase 1") {
		t.Errorf("message = %q, want the phase", stderr)
	}
}

func TestCARotateRefusesAPhaseThatIsNotBuilt(t *testing.T) {
	dir := writeApplyMesh(t)

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "phase 2") {
		t.Errorf("message = %q, want the phase", stderr)
	}
}

func TestCARotateIsInTheDispatchAndTheUsage(t *testing.T) {
	out, _, code := captureBoth(t, func() int { return run([]string{"help"}) })
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out, "ca rotate  rotate the trust anchor") {
		t.Errorf("the usage does not list ca rotate:\n%s", out)
	}
	if strings.Contains(out, "status, ca rotate") {
		t.Error("the usage still lists ca rotate as planned")
	}
}
