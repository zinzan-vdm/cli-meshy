package bundle_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/bundle"
)

// unpack reads every member of an archive into a map from name to body and
// keeps the mode of each header.
func unpack(t *testing.T, body []byte) (map[string][]byte, map[string]int64) {
	t.Helper()

	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("the archive is not gzip: %v", err)
	}
	defer gz.Close()

	files := map[string][]byte{}
	modes := map[string]int64{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read a header: %v", err)
		}
		if strings.Contains(header.Name, "/") {
			t.Fatalf("%s is not at the top level", header.Name)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		files[header.Name] = content
		modes[header.Name] = header.Mode
	}
	return files, modes
}

func sample() bundle.Archive {
	return bundle.Archive{
		Mesh:       "homelab",
		Node:       "laptop-1",
		Config:     []byte("pki:\n  ca: /etc/nebula/meshy-homelab/ca.crt\n"),
		Authority:  []byte("CA CRT\n"),
		Cert:       []byte("LAPTOP CRT\n"),
		Key:        []byte("LAPTOP KEY\n"),
		Blocklist:  []byte("4f3a\n"),
		Binary:     []byte("\x7fELF that is not really a binary"),
		BinaryName: "nebula",
		Installers: map[string][]byte{
			"install.sh":      []byte("#!/bin/sh\necho linux\n"),
			"install.command": []byte("#!/bin/sh\necho macos\n"),
			"install.ps1":     []byte("Write-Output 'windows'\n"),
		},
		Readme: bundle.Readme("homelab", "laptop-1"),
	}
}

func TestBuildWritesEveryFileFlat(t *testing.T) {
	files, modes := unpack(t, mustBuild(t, sample()))

	want := []string{
		"blocklist.txt", "config.yml", "ca.crt", "laptop-1.crt", "laptop-1.key",
		"nebula", "install.command", "install.ps1", "install.sh", "README.txt",
	}
	if len(files) != len(want) {
		t.Fatalf("the archive holds %d files, want %d: %v", len(files), len(want), names(files))
	}
	for _, name := range want {
		if _, ok := files[name]; !ok {
			t.Errorf("the archive holds no %s", name)
		}
	}

	if string(files["laptop-1.key"]) != "LAPTOP KEY\n" {
		t.Error("the key body changed")
	}
	if modes["laptop-1.key"] != 0o600 {
		t.Errorf("the key has mode %o, want 0600", modes["laptop-1.key"])
	}
	if modes["config.yml"] != 0o600 {
		t.Errorf("the config has mode %o, want 0600", modes["config.yml"])
	}
	if modes["nebula"] != 0o755 {
		t.Errorf("the binary has mode %o, want 0755", modes["nebula"])
	}
	if modes["install.sh"] != 0o755 {
		t.Errorf("install.sh has mode %o, want 0755", modes["install.sh"])
	}
	if modes["install.command"] != 0o755 {
		t.Errorf("install.command has mode %o, want 0755", modes["install.command"])
	}
}

func TestBuildLeavesTheBinaryOutWhenItIsNil(t *testing.T) {
	held := sample()
	held.Binary = nil

	files, _ := unpack(t, mustBuild(t, held))
	if _, ok := files["nebula"]; ok {
		t.Error("the archive holds a binary that the caller left out")
	}
	if _, ok := files["laptop-1.key"]; !ok {
		t.Error("the archive dropped a file that it always holds")
	}
}

func TestBuildRefusesAnArchiveWithNoInstaller(t *testing.T) {
	held := sample()
	delete(held.Installers, "install.ps1")

	if _, err := bundle.Build(held); err == nil {
		t.Fatal("Build accepted an archive with no install.ps1")
	}
}

func TestBuildNeedsAMeshAndANode(t *testing.T) {
	held := sample()
	held.Node = ""

	if _, err := bundle.Build(held); err == nil {
		t.Fatal("Build accepted an archive with no node name")
	}
}

func TestSameInputMakesTheSameBytes(t *testing.T) {
	first := mustBuild(t, sample())
	second := mustBuild(t, sample())

	if !bytes.Equal(first, second) {
		t.Error("two builds of the same input differ")
	}
}

func TestWriteSetsTheModeAndMakesTheDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundles", "homelab", "laptop-1.tar.gz")

	if err := bundle.Write(path, []byte("archive")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the archive has mode %o, want 0600", info.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "archive" {
		t.Errorf("the archive holds %q", body)
	}
}

func TestReadmeHasFiveLines(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(string(bundle.Readme("homelab", "laptop-1")), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("the README has %d lines, want 5: %q", len(lines), lines)
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Errorf("line %d is empty", i+1)
		}
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"tar xzf laptop-1.tar.gz",
		"install.sh on Linux",
		"install.command on macOS",
		"install.ps1 on Windows",
		"sudo",
		"unsigned",
		"meshy-homelab is running",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the README does not say %q", want)
		}
	}
}

func mustBuild(t *testing.T, held bundle.Archive) []byte {
	t.Helper()
	body, err := bundle.Build(held)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	return body
}

func names(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for name := range files {
		out = append(out, name)
	}
	return out
}
