package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/bootstrap"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

const bootstrapMesh = `schema: 1
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
certificates:
  server: 1y
  user: 1y
  renew_before: 60d
nodes:
  - name: core-1
    kind: server
    groups: [core]
    ip: 10.42.0.1
    ssh:
      user: meshy
      host: 203.0.113.10
    public_addr: 203.0.113.10:4242
  - name: web-1
    kind: server
    groups: [web]
    ip: 10.42.0.11
    ssh:
      user: meshy
      host: 203.0.113.11
  - name: horo-laptop
    kind: user
    groups: [people]
    ip: 10.42.0.50
    platform: darwin/arm64
rules: []
`

func writeBootstrapMesh(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

func TestBootstrapPrintsOnlyAScript(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	body, code := capture(t, func() int { return cmdBootstrap([]string{"web-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}
	if !strings.HasPrefix(body, "#!/bin/sh\n") {
		t.Errorf("the output does not start with a shebang:\n%s", body)
	}
	if !strings.HasSuffix(body, "echo \"node web-1 is prepared for mesh homelab\"\n") {
		t.Errorf("the output does not end with the last line of the script:\n%s", body)
	}

	// The command prints and adds nothing, so a maintainer can pipe it straight
	// into sudo.
	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load mesh: %v", err)
	}
	want, err := bootstrap.Script(*mesh, mesh.Nodes[1])
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	if body != want {
		t.Errorf("the output is not the script alone:\n%s", body)
	}
}

func TestBootstrapRefusesAUserNode(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	body, code := capture(t, func() int { return cmdBootstrap([]string{"horo-laptop", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if body != "" {
		t.Errorf("a refused node printed a script:\n%s", body)
	}
}

func TestBootstrapRefusesAUserNodeWithItsOwnMessage(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	msg, code := captureErr(t, func() int { return cmdBootstrap([]string{"horo-laptop", "--mesh", dir}) })
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if want := "horo-laptop is a user node, run meshy bundle\n"; msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestBootstrapRefusesANodeThatIsNotInTheMesh(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	msg, code := captureErr(t, func() int { return cmdBootstrap([]string{"web-9", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(msg, "web-9") {
		t.Errorf("the message does not name the node: %q", msg)
	}
}

func TestBootstrapRefusesAMeshFileThatDoesNotLoad(t *testing.T) {
	dir := t.TempDir()

	msg, code := captureErr(t, func() int { return cmdBootstrap([]string{"web-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(msg, "mesh.yml") {
		t.Errorf("the message does not name the file: %q", msg)
	}
}

func TestBootstrapRefusesAMeshFileWithProblems(t *testing.T) {
	dir := writeBootstrapMesh(t, strings.Replace(bootstrapMesh, "ip: 10.42.0.11", "ip: 10.0.0.11", 1))

	_, code := captureErr(t, func() int { return cmdBootstrap([]string{"web-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestBootstrapNeedsANodeName(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	msg, code := captureErr(t, func() int { return cmdBootstrap([]string{"--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(msg, "name one node") {
		t.Errorf("the message does not say what to do: %q", msg)
	}
}

func TestBootstrapNamesTheUnitThatPlanReads(t *testing.T) {
	dir := writeBootstrapMesh(t, bootstrapMesh)

	body, code := capture(t, func() int { return cmdBootstrap([]string{"web-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, body)
	}
	// These two constants are the contract with plan and apply.
	if !strings.Contains(body, "nebula@meshy-homelab.service") {
		t.Error("the script does not write the unit that apply reloads")
	}
	if !strings.Contains(body, "/etc/nebula/meshy-homelab") {
		t.Error("the script does not use the config directory that apply writes")
	}
}
