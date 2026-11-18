package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/apply"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// captureBoth runs fn and returns what it wrote to stdout and stderr.
func captureBoth(t *testing.T, fn func() int) (string, string, int) {
	t.Helper()

	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW

	code := fn()

	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr

	out, err := io.ReadAll(outR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errOut, err := io.ReadAll(errR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(out), string(errOut), code
}

// writeApplyMesh writes the mesh of the issue tests into a new directory.
func writeApplyMesh(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(issueMesh), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

// openApplyVault seals a vault that holds the authority and the named nodes,
// then opens it, which is the state that apply needs.
func openApplyVault(t *testing.T, meshDir string, names ...string) {
	t.Helper()

	record := vault.Record{
		"ca-1.crt": []byte("authority certificate\n"),
		"ca-1.key": []byte("authority key\n"),
	}
	for _, name := range names {
		vault.SetCertificate(record, name, []byte(name+" certificate\n"), []byte(name+" key\n"))
	}

	plaintext, err := vault.Encode(record)
	if err != nil {
		t.Fatalf("encode the vault: %v", err)
	}
	sealed, shares, err := vault.Seal(plaintext)
	if err != nil {
		t.Fatalf("seal the vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, vault.FileName), sealed, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}
	if _, err := vault.Unlock(meshDir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock the vault: %v", err)
	}
}

// fakeNode is one node with a filesystem in memory. It answers the commands that
// a run builds, so it cannot drift from the commands that a run sends. Prints
// holds the answer of `nebula-cert print` for a path, which is how a test sets
// the authority list that the node reports.
type fakeNode struct {
	name   string
	files  map[string]fakeFile
	prints map[string]string
	// printsAfter holds the answer that the node gives for a path once the run
	// has written that path, so a read back sees what the write changed.
	printsAfter map[string]string
	noUnit      bool
	failAll     bool
	log         []string
	writes      int
	reloads     int
	restarts    int
}

type fakeFile struct {
	body []byte
	mode fs.FileMode
}

func newFakeNode(name string) *fakeNode {
	return &fakeNode{
		name:        name,
		files:       map[string]fakeFile{},
		prints:      map[string]string{},
		printsAfter: map[string]string{},
	}
}

func (f *fakeNode) Run(_ context.Context, cmd string) (node.Result, error) {
	f.log = append(f.log, cmd)

	if f.failAll {
		return node.Result{ExitCode: 1, Stderr: "connection reset"}, nil
	}
	switch {
	case strings.HasPrefix(cmd, "systemctl cat "):
		if f.noUnit {
			return node.Result{ExitCode: 1, Stderr: "No files found for nebula@meshy-homelab.service."}, nil
		}
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "systemctl is-active "):
		return node.Result{Stdout: "active\n"}, nil
	case strings.HasPrefix(cmd, "sudo -n systemctl reload "):
		f.reloads++
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "sudo -n systemctl restart "):
		f.restarts++
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "mv -f "):
		from, to := movePaths(cmd)
		file, ok := f.files[from]
		if !ok {
			return node.Result{ExitCode: 1, Stderr: "No such file or directory"}, nil
		}
		delete(f.files, from)
		f.files[to] = file
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "sha256sum "):
		path := unquote(strings.TrimPrefix(cmd, "sha256sum "))
		file, ok := f.files[path]
		if !ok {
			return node.Result{ExitCode: 1, Stderr: "No such file or directory"}, nil
		}
		sum := sha256.Sum256(file.body)
		return node.Result{Stdout: hex.EncodeToString(sum[:]) + "  " + path + "\n"}, nil
	case strings.HasPrefix(cmd, "nebula-cert print -json -path "):
		path := unquote(strings.TrimPrefix(cmd, "nebula-cert print -json -path "))
		if out, ok := f.printsAfter[path]; ok {
			if _, written := f.files[path]; written {
				return node.Result{Stdout: out}, nil
			}
		}
		out, ok := f.prints[path]
		if !ok {
			return node.Result{ExitCode: 1, Stderr: "open " + path + ": no such file or directory"}, nil
		}
		return node.Result{Stdout: out}, nil
	}
	return node.Result{ExitCode: 127, Stderr: "command not found: " + cmd}, nil
}

func (f *fakeNode) Write(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	f.writes++
	f.files[path] = fakeFile{body: append([]byte(nil), data...), mode: mode}
	return nil
}

func (f *fakeNode) Read(_ context.Context, path string) ([]byte, error) {
	file, ok := f.files[path]
	if !ok {
		return nil, node.ErrNoFile
	}
	return file.body, nil
}

func (f *fakeNode) read(path string) string {
	file, ok := f.files[path]
	if !ok {
		return ""
	}
	return string(file.body)
}

// useFakes makes every dial return the fake that the node name selects. It
// returns the map so a test can read the state that the run left.
func useFakes(t *testing.T, names ...string) map[string]*fakeNode {
	t.Helper()

	nodes := make(map[string]*fakeNode, len(names))
	for _, name := range names {
		nodes[name] = newFakeNode(name)
	}

	old := dialSession
	dialSession = func(n config.Node, _ string) (apply.Session, error) {
		fake, ok := nodes[n.Name]
		if !ok {
			return nil, errors.New("no fake for " + n.Name)
		}
		return fake, nil
	}
	t.Cleanup(func() { dialSession = old })
	return nodes
}

func movePaths(cmd string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(cmd, "mv -f "), "' '")
	if len(parts) != 2 {
		return "", ""
	}
	return unquote(parts[0]), unquote(parts[1])
}

func unquote(s string) string { return strings.Trim(s, "'") }

func TestApplyIsInTheDispatchAndTheUsage(t *testing.T) {
	out, _, code := captureBoth(t, func() int { return run([]string{"help"}) })
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out, "apply      make every node match mesh.yml") {
		t.Errorf("the usage does not list apply:\n%s", out)
	}
	if strings.Contains(out, "apply, bundle") {
		t.Error("the usage still lists apply as planned")
	}
}

func TestApplyRefusesAnUnknownNode(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "gone-1"}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, `node "gone-1" is not in mesh.yml`) {
		t.Errorf("the error does not name the node: %q", errOut)
	}
}

func TestApplyRefusesAUserNode(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "laptop-1"}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "laptop-1 is a user node, run meshy bundle") {
		t.Errorf("the error does not send the user to the bundle: %q", errOut)
	}
}

func TestApplyRefusesAMeshThatDoesNotLoad(t *testing.T) {
	dir := t.TempDir()

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, config.FileName) {
		t.Errorf("the error does not name the mesh file: %q", errOut)
	}
}

func TestApplyRefusesAMeshWithProblems(t *testing.T) {
	dir := writeApplyMesh(t)
	path := filepath.Join(dir, config.FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	broken := strings.Replace(string(body), "name: homelab", "name: Bad_Name", 1)
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}

	out, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
}

func TestApplySaysUnlockWhenTheVaultIsClosed(t *testing.T) {
	dir := writeApplyMesh(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "meshy unlock") {
		t.Errorf("the error does not send the maintainer to unlock: %q", errOut)
	}
}

func TestApplyRefusesWithoutKnownHosts(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "MESHY_KNOWN_HOSTS") {
		t.Errorf("the error does not name the variable: %q", errOut)
	}
}

func TestApplyAppliesEveryServerNodeAndSkipsAUserNode(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	for _, name := range []string{"core-1", "web-1"} {
		if !strings.Contains(out, name+": wrote blocklist.txt config.yml") {
			t.Errorf("the report does not name %s: %q", name, out)
		}
		if nodes[name].reloads != 1 {
			t.Errorf("%s reloaded %d times, want 1", name, nodes[name].reloads)
		}
	}
	if strings.Contains(out, "laptop-1") {
		t.Errorf("the report names a user node: %q", out)
	}
	if !strings.Contains(out, "reloaded") {
		t.Errorf("the report does not name the reload: %q", out)
	}
}

func TestApplyLimitsTheRunToTheNodeThatTheFlagNames(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")

	out, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "web-1"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(out, "core-1") {
		t.Errorf("the run reached a node that the flag did not name: %q", out)
	}
	if len(nodes["core-1"].log) != 0 {
		t.Errorf("core-1 saw %d commands, want none", len(nodes["core-1"].log))
	}
}

func TestApplyContinuesAfterANodeFails(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	nodes["core-1"].failAll = true

	out, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "core-1: failed: ") {
		t.Errorf("the report does not name the failed node: %q", out)
	}
	if !strings.Contains(out, "web-1: wrote blocklist.txt") {
		t.Errorf("the run stopped before the second node: %q", out)
	}
	if nodes["web-1"].reloads != 1 {
		t.Errorf("web-1 reloaded %d times, want 1", nodes["web-1"].reloads)
	}
}

func TestApplyNamesBootstrapWhenTheUnitIsMissing(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")
	nodes["core-1"].noUnit = true

	out, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "run meshy bootstrap core-1") {
		t.Errorf("the report does not name the bootstrap: %q", out)
	}
}

func TestApplyRenewsACertificateInsideTheWindow(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(10*24*time.Hour))
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	if !strings.Contains(out, "core-1: renewed, wrote") {
		t.Errorf("the report does not name the renewal: %q", out)
	}
	if !strings.Contains(nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt"), "STUB CRT") {
		t.Error("a renewed certificate did not reach the node")
	}
}

func TestApplyLeavesACertificateOutsideTheWindow(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	if strings.Contains(out, "renewed") {
		t.Errorf("the report names a renewal that was not due: %q", out)
	}
	if got := nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt"); got != "core-1 certificate\n" {
		t.Errorf("the node holds %q, want the certificate from the vault", got)
	}
}

func TestApplyRevokesANodeThatLeftTheMesh(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1", "old-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}
	if !strings.Contains(out, "revoked old-1 (4f3a)") {
		t.Errorf("the report does not name the revocation: %q", out)
	}
	for _, name := range []string{"core-1", "web-1"} {
		body := nodes[name].read("/etc/nebula/meshy-homelab/blocklist.txt")
		if !strings.Contains(body, "4f3a") {
			t.Errorf("%s holds the blocklist %q, want the fingerprint", name, body)
		}
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if _, _, found := vault.Certificate(record, "old-1"); found {
		t.Error("the vault still holds the certificate of the revoked node")
	}
	if _, _, found := vault.Certificate(record, "core-1"); !found {
		t.Error("the run removed a node that the mesh still names")
	}
}

func TestApplyKeepsTheVaultWhenANodeFails(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1", "old-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	nodes["core-1"].failAll = true

	_, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if _, _, found := vault.Certificate(record, "old-1"); !found {
		t.Error("the vault dropped the certificate before every node took the blocklist")
	}
}

func TestApplySecondRunWritesNothing(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")

	if _, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) }); code != 0 {
		t.Fatalf("first run exit = %d, want 0", code)
	}
	before := nodes["core-1"].writes

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 0 {
		t.Fatalf("second run exit = %d, want 0: %s", code, errOut)
	}
	if out != "core-1: unchanged\n" {
		t.Errorf("stdout = %q, want the unchanged line", out)
	}
	if nodes["core-1"].writes != before {
		t.Errorf("the second run wrote %d files, want none", nodes["core-1"].writes-before)
	}
}

func TestApplyDryRunPrintsThePlanAndWritesNothing(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	nodes := useFakes(t, "core-1")

	// The plan needs a host key, so the run stops with the message that plan
	// writes. That message proves that apply handed the run to plan.
	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--dry-run"}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "meshy plan:") {
		t.Errorf("the run did not hand the dry run to plan: %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if len(nodes["core-1"].log) != 0 {
		t.Errorf("the dry run sent %d commands to a node", len(nodes["core-1"].log))
	}
}

func TestApplyPrintsTheReportAsJSON(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	useFakes(t, "core-1", "web-1")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--json"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	var report apply.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, out)
	}
	if report.Mesh != "homelab" {
		t.Errorf("mesh = %q, want homelab", report.Mesh)
	}
	if len(report.Nodes) != 2 {
		t.Fatalf("the report holds %d nodes, want 2", len(report.Nodes))
	}
	if report.Nodes[0].Name != "core-1" || report.Nodes[1].Name != "web-1" {
		t.Errorf("the report names %v", report.Nodes)
	}
	if report.Nodes[0].Action != apply.ActionReload {
		t.Errorf("action = %q, want reload", report.Nodes[0].Action)
	}
	for _, n := range report.Nodes {
		if len(n.Writes) == 0 {
			t.Errorf("%s reports no write", n.Name)
		}
	}
}
