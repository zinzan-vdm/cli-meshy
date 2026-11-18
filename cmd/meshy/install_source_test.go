package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/install"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// sourceDir writes the two files that the project publishes, so a run with
// --source needs no network.
func sourceDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for _, member := range []struct{ name, body string }{
		{"nebula", "the nebula binary"},
		{"nebula-cert", "the signing tool"},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: member.name, Mode: 0o755, Size: int64(len(member.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(member.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	asset := "nebula-linux-amd64.tar.gz"
	sum := sha256.Sum256(buf.Bytes())
	binary := sha256.Sum256([]byte("the nebula binary"))
	sums := hex.EncodeToString(sum[:]) + "  " + asset + "\n" +
		hex.EncodeToString(binary[:]) + "  " + asset + "/nebula\n"

	if err := os.WriteFile(filepath.Join(dir, asset), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SHASUM256.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sourceMesh writes the smallest mesh that holds one server node.
func sourceMesh(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	body := `name: homelab
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
    public_addr: 203.0.113.10:4242
    ssh:
      user: meshy
      host: 203.0.113.10
`
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sourceNode is one node that answers the commands and keeps the bytes.
type sourceNode struct {
	log    []string
	placed []byte
}

func (s *sourceNode) Run(_ context.Context, cmd string) (node.Result, error) {
	switch {
	case cmd == "uname -s":
		return node.Result{Stdout: "Linux\n"}, nil
	case cmd == "uname -m":
		return node.Result{Stdout: "x86_64\n"}, nil
	case cmd == "nebula -version":
		s.log = append(s.log, "version")
		if len(s.placed) == 0 {
			return node.Result{ExitCode: 127, Stderr: "nebula: not found"}, nil
		}
		return node.Result{Stdout: "Version: 1.12.0\n"}, nil
	case strings.HasPrefix(cmd, "sudo -n install "):
		s.log = append(s.log, "place")
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "sudo -n systemctl restart "):
		s.log = append(s.log, "restart")
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "rm -f "):
		return node.Result{}, nil
	}
	return node.Result{ExitCode: 127, Stderr: "command not found: " + cmd}, nil
}

func (s *sourceNode) Write(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	if path != node.Dir("homelab")+"/"+install.Incoming {
		return fs.ErrInvalid
	}
	if mode.Perm() != 0o700 {
		return fs.ErrInvalid
	}
	s.placed = append([]byte(nil), data...)
	return nil
}

func TestCmdInstallRunsOneNodeFromASource(t *testing.T) {
	meshDir := sourceMesh(t)
	source := sourceDir(t)
	remote := &sourceNode{}

	was := installOpen
	installOpen = func(string) func(config.Node) (install.Session, error) {
		return func(config.Node) (install.Session, error) { return remote, nil }
	}
	t.Cleanup(func() { installOpen = was })
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(t.TempDir(), "known_hosts"))

	out, errOut, code := captureBoth(t, func() int {
		return cmdInstall([]string{"--mesh", meshDir, "--source", source})
	})
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, errOut)
	}
	if want := "core-1: v1.12.0, was nothing, restarted nebula@meshy-homelab\n"; out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}

	cached := filepath.Join(meshDir, ".meshy", "bin", "v1.12.0", "linux-amd64", "nebula")
	body, err := os.ReadFile(cached)
	if err != nil {
		t.Fatalf("the cache holds no binary: %v", err)
	}
	if string(body) != "the nebula binary" {
		t.Errorf("the cache holds %q", body)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cached), "nebula-cert")); err == nil {
		t.Error("the cache holds nebula-cert")
	}
	if string(remote.placed) != "the nebula binary" {
		t.Errorf("the node holds %q", remote.placed)
	}
}

func TestCmdInstallNeedsKnownHosts(t *testing.T) {
	meshDir := sourceMesh(t)
	t.Setenv("MESHY_KNOWN_HOSTS", "")

	_, errOut, code := captureBoth(t, func() int {
		return cmdInstall([]string{"--mesh", meshDir, "--source", sourceDir(t)})
	})
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "MESHY_KNOWN_HOSTS is not set") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCmdInstallRefusesAStrayArgument(t *testing.T) {
	_, _, code := captureBoth(t, func() int {
		return cmdInstall([]string{"core-1"})
	})
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}

func TestCmdInstallFailsTheRunWhenANodeFails(t *testing.T) {
	meshDir := sourceMesh(t)

	was := installOpen
	installOpen = func(string) func(config.Node) (install.Session, error) {
		return func(config.Node) (install.Session, error) {
			return nil, os.ErrPermission
		}
	}
	t.Cleanup(func() { installOpen = was })
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(t.TempDir(), "known_hosts"))

	_, errOut, code := captureBoth(t, func() int {
		return cmdInstall([]string{"--mesh", meshDir, "--source", sourceDir(t)})
	})
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "node core-1:") {
		t.Errorf("stderr = %q", errOut)
	}
}
