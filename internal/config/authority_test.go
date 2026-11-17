package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMeshFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

func TestSetAuthorityCreatesTheBlock(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\ncidr: 10.42.0.0/24\n")

	changed, err := SetAuthority(dir, "ca-1")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if !changed {
		t.Fatal("set reported no change")
	}

	mesh, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 1 || mesh.CA.Trust[0] != "ca-1" {
		t.Errorf("trust = %v, want [ca-1]", mesh.CA.Trust)
	}
	if mesh.CA.Active != "ca-1" {
		t.Errorf("active = %q, want ca-1", mesh.CA.Active)
	}
}

func TestSetAuthorityFillsAGapInAnExistingBlock(t *testing.T) {
	dir := writeMeshFile(t, `schema: 1
name: homelab
cidr: 10.42.0.0/24
ca:
  duration: 20y
`)

	if _, err := SetAuthority(dir, "ca-1"); err != nil {
		t.Fatalf("set: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "duration: 20y") {
		t.Errorf("the existing key was lost:\n%s", body)
	}
	if !strings.Contains(string(body), "active: ca-1") {
		t.Errorf("active was not added:\n%s", body)
	}
}

func TestSetAuthorityLeavesACompleteFileAlone(t *testing.T) {
	body := `schema: 1
name: homelab
cidr: 10.42.0.0/24
ca:
  duration: 20y
  trust: [ca-1]
  active: ca-1
`
	dir := writeMeshFile(t, body)
	path := filepath.Join(dir, FileName)

	changed, err := SetAuthority(dir, "ca-1")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if changed {
		t.Error("set reported a change on a complete file")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(after) != body {
		t.Errorf("the file changed:\n%s", after)
	}
}

func TestSetAuthorityKeepsComments(t *testing.T) {
	dir := writeMeshFile(t, `schema: 1
name: homelab
cidr: 10.42.0.0/24
# The authority outlives every node.
ca:
  duration: 20y
`)

	if _, err := SetAuthority(dir, "ca-1"); err != nil {
		t.Fatalf("set: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "# The authority outlives every node.") {
		t.Errorf("the comment was lost:\n%s", body)
	}
}
