package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const trustMesh = `schema: 1

# The mesh that this repository describes.
name: homelab
cidr: 10.42.0.0/24

ca:
  # The authority outlives every node.
  duration: 20y
  rotate_at: 80%
  trust: [ca-1]  # the authority that signs
  active: ca-1

lighthouses:
  - node: core-1
    port: 4242

rules: []
`

func TestAddTrustChangesOneLineAndKeepsTheRest(t *testing.T) {
	dir := writeMeshFile(t, trustMesh)

	changed, err := AddTrust(dir, "ca-2")
	if err != nil {
		t.Fatalf("AddTrust: %v", err)
	}
	if !changed {
		t.Fatal("AddTrust reported no change")
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := strings.Replace(trustMesh, "trust: [ca-1]  # the authority that signs", "trust: [ca-1, ca-2]  # the authority that signs", 1)
	if string(body) != want {
		t.Errorf("the file changed by more than the trust list:\n%s", body)
	}
}

func TestAddTrustKeepsEveryCharacterThatItDidNotEdit(t *testing.T) {
	dir := writeMeshFile(t, trustMesh)

	if _, err := AddTrust(dir, "ca-2"); err != nil {
		t.Fatalf("AddTrust: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, keep := range []string{
		"# The mesh that this repository describes.",
		"# The authority outlives every node.",
		"# the authority that signs",
		"\n\nca:\n",
		"\nlighthouses:\n",
	} {
		if !strings.Contains(string(body), keep) {
			t.Errorf("the edit dropped %q:\n%s", keep, body)
		}
	}
}

func TestAddTrustLeavesAFileThatAlreadyNamesTheAuthority(t *testing.T) {
	dir := writeMeshFile(t, trustMesh)

	changed, err := AddTrust(dir, "ca-1")
	if err != nil {
		t.Fatalf("AddTrust: %v", err)
	}
	if changed {
		t.Error("AddTrust reported a change for a name that is already trusted")
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != trustMesh {
		t.Errorf("the file changed:\n%s", body)
	}
}

func TestAddTrustReadsABlockList(t *testing.T) {
	dir := writeMeshFile(t, `schema: 1
name: homelab
ca:
  duration: 20y
  trust:
    - ca-1
  active: ca-1
rules: []
`)

	if _, err := AddTrust(dir, "ca-2"); err != nil {
		t.Fatalf("AddTrust: %v", err)
	}

	mesh, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 2 || mesh.CA.Trust[1] != "ca-2" {
		t.Errorf("trust = %v, want [ca-1 ca-2]", mesh.CA.Trust)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := `  trust:
    - ca-1
    - ca-2
  active: ca-1
`
	if !strings.Contains(string(body), want) {
		t.Errorf("the block list is not in the form a person wrote:\n%s", body)
	}
}

func TestAddTrustFillsAnEmptyList(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  trust: []\n  active: ca-1\n")

	if _, err := AddTrust(dir, "ca-2"); err != nil {
		t.Fatalf("AddTrust: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "trust: [ca-2]") {
		t.Errorf("the list is not filled:\n%s", body)
	}
}

func TestAddTrustRefusesAFileWithoutATrustList(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  duration: 20y\n")

	_, err := AddTrust(dir, "ca-2")
	if err == nil {
		t.Fatal("AddTrust accepted a file with no ca.trust")
	}
	if !strings.Contains(err.Error(), "ca.trust") {
		t.Errorf("error does not name the list: %v", err)
	}
}
