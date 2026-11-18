package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetActiveChangesOneLineAndKeepsTheRest(t *testing.T) {
	dir := writeMeshFile(t, trustMesh)

	changed, err := SetActive(dir, "ca-2")
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if !changed {
		t.Fatal("SetActive reported no change")
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := strings.Replace(trustMesh, "active: ca-1", "active: ca-2", 1)
	if string(body) != want {
		t.Errorf("the file changed by more than the active line:\n%s", body)
	}
	for _, keep := range []string{
		"# The mesh that this repository describes.",
		"# The authority outlives every node.",
		"# the authority that signs",
		"trust: [ca-1]",
		"\n\nca:\n",
		"\nlighthouses:\n",
	} {
		if !strings.Contains(string(body), keep) {
			t.Errorf("the edit dropped %q:\n%s", keep, body)
		}
	}

	mesh, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-2" {
		t.Errorf("active = %q, want ca-2", mesh.CA.Active)
	}
}

func TestSetActiveKeepsTheSpacingAndTheComment(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  trust: [ca-1]\n  active:   ca-1  # the signer\nrules: []\n")

	if _, err := SetActive(dir, "ca-2"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "  active:   ca-2  # the signer\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("the line is not in the form a person wrote:\n%s", body)
	}
}

func TestSetActiveKeepsTheQuotes(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  trust: [\"ca-1\"]\n  active: \"ca-1\"\n")

	if _, err := SetActive(dir, "ca-2"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), `active: "ca-2"`) {
		t.Errorf("the edit dropped the quotes:\n%s", body)
	}
}

func TestSetActiveLeavesAFileThatAlreadyNamesTheAuthority(t *testing.T) {
	dir := writeMeshFile(t, trustMesh)

	changed, err := SetActive(dir, "ca-1")
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if changed {
		t.Error("SetActive reported a change for a name that is already active")
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != trustMesh {
		t.Errorf("the file changed:\n%s", body)
	}
}

func TestSetActiveWritesTheLineThatTheFileLeftOut(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  trust: [ca-1, ca-2]\nrules: []\n")

	changed, err := SetActive(dir, "ca-2")
	if err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if !changed {
		t.Fatal("SetActive reported no change")
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "  trust: [ca-1, ca-2]\n  active: ca-2\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("the line is not after the trust list:\n%s", body)
	}

	mesh, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-2" {
		t.Errorf("active = %q, want ca-2", mesh.CA.Active)
	}
}

func TestSetActiveRefusesAFileWithoutACABlock(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\n")

	_, err := SetActive(dir, "ca-2")
	if err == nil {
		t.Fatal("SetActive accepted a file with no ca block")
	}
	if !strings.Contains(err.Error(), "ca block") {
		t.Errorf("error does not name the block: %v", err)
	}
}

func TestSetActiveWritesTheLineAfterABlockList(t *testing.T) {
	dir := writeMeshFile(t, "schema: 1\nname: homelab\nca:\n  trust:\n    - ca-1\n    - ca-2\nrules: []\n")

	if _, err := SetActive(dir, "ca-2"); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "  trust:\n    - ca-1\n    - ca-2\n  active: ca-2\nrules: []\n"
	if !strings.Contains(string(body), want) {
		t.Errorf("the line landed inside the block list:\n%s", body)
	}

	mesh, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-2" || len(mesh.CA.Trust) != 2 {
		t.Errorf("active = %q, trust = %v", mesh.CA.Active, mesh.CA.Trust)
	}
}
