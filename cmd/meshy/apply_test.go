package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/apply"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
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

func TestApplyIsInTheDispatchAndTheUsage(t *testing.T) {
	out, _, code := captureBoth(t, func() int { return run([]string{"help"}) })
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out, "apply      write the config and the certificate to one node") {
		t.Errorf("the usage does not list apply:\n%s", out)
	}
	if strings.Contains(out, "apply, bundle") {
		t.Error("the usage still lists apply as planned")
	}
}

func TestApplyRefusesWithoutANode(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--node") {
		t.Errorf("the error does not name the flag: %q", errOut)
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

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
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

	out, _, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
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

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "meshy unlock") {
		t.Errorf("the error does not send the maintainer to unlock: %q", errOut)
	}
}

func TestApplySaysIssueWhenTheNodeHasNoCertificate(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "run meshy issue core-1") {
		t.Errorf("the error does not send the maintainer to issue: %q", errOut)
	}
}

func TestApplyFilesTakesTheVaultMaterialAndTheRenderedConfig(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	target, found := applyTarget(*mesh, "core-1")
	if !found {
		t.Fatal("core-1 is not in the mesh")
	}

	files, code := applyFiles(*mesh, target, dir)
	if code != 0 {
		t.Fatalf("applyFiles exit = %d, want 0", code)
	}
	if string(files.Authority) != "authority certificate\n" {
		t.Errorf("authority = %q", files.Authority)
	}
	if string(files.Cert) != "core-1 certificate\n" || string(files.Key) != "core-1 key\n" {
		t.Errorf("certificate = %q, key = %q", files.Cert, files.Key)
	}

	rendered, err := nebula.Render(*mesh, target)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(files.Config) != string(rendered) {
		t.Error("the config is not the rendered config")
	}
}

// noUnitSession answers every command as a node that was never bootstrapped.
type noUnitSession struct{}

func (noUnitSession) Run(context.Context, string) (node.Result, error) {
	return node.Result{ExitCode: 1, Stderr: "No files found for nebula@meshy-homelab.service."}, nil
}

func (noUnitSession) Read(context.Context, string) ([]byte, error) {
	return nil, errors.New("the run read a file from a node with no unit")
}

func (noUnitSession) Write(context.Context, string, []byte, fs.FileMode) error {
	return errors.New("the run wrote to a node with no unit")
}

func TestApplyExitsOneAndNamesBootstrapWhenTheUnitIsMissing(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	old := dialSession
	dialSession = func(config.Node, string) (apply.Session, error) { return noUnitSession{}, nil }
	t.Cleanup(func() { dialSession = old })

	out, errOut, code := captureBoth(t, func() int { return run([]string{"apply", "--mesh", dir, "--node", "core-1"}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "run meshy bootstrap core-1") {
		t.Errorf("the error does not name the bootstrap command: %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
}

func TestReportOneNamesTheFilesAndTheReload(t *testing.T) {
	mesh := config.Mesh{Name: "homelab"}
	target := config.Node{Name: "core-1"}

	out, _, _ := captureBoth(t, func() int {
		reportOne(target, mesh, apply.Result{Writes: []string{"config.yml", "core-1.crt"}, Reloaded: true})
		return 0
	})
	want := "core-1: wrote config.yml core-1.crt\ncore-1: reloaded nebula@meshy-homelab\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestReportOneSaysUnchangedWhenNothingWasWritten(t *testing.T) {
	mesh := config.Mesh{Name: "homelab"}
	target := config.Node{Name: "core-1"}

	out, _, _ := captureBoth(t, func() int {
		reportOne(target, mesh, apply.Result{})
		return 0
	})
	if out != "core-1: unchanged\n" {
		t.Errorf("stdout = %q", out)
	}
}
