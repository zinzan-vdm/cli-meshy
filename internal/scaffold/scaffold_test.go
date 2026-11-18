package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/scaffold"
)

func TestInitWritesAValidMesh(t *testing.T) {
	dir := t.TempDir()

	written, err := scaffold.Init(scaffold.Options{Dir: dir, Name: "homelab", CIDR: "10.42.0.0/24"})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if len(written) != 4 {
		t.Fatalf("expected 4 files, got %v", written)
	}

	for _, path := range []string{
		config.FileName,
		".gitignore",
		filepath.Join(".github", "CODEOWNERS"),
		filepath.Join(".github", "workflows", "meshy.yml"),
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("%s was not written: %v", path, err)
		}
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if problems := mesh.Validate(); len(problems) != 0 {
		t.Fatalf("the mesh that init wrote is not valid: %v", problems)
	}
	if mesh.Interface != config.GenerateInterface("homelab") {
		t.Fatalf("interface = %q, want the generated default", mesh.Interface)
	}
	if mesh.Certificates.Server.Duration != mesh.Certificates.User.Duration {
		t.Fatalf("certificate lifetimes should default to the same value")
	}
}

func TestInitDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffold.Init(scaffold.Options{Dir: dir, Name: "homelab"}); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	written, err := scaffold.Init(scaffold.Options{Dir: dir, Name: "homelab"})
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	if len(written) != 0 {
		t.Fatalf("second Init overwrote or rewrote %v", written)
	}
}

func TestInitKeepsTheBundlesOutOfGit(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffold.Init(scaffold.Options{Dir: dir, Name: "homelab"}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, want := range []string{".meshy/", "bundles/", "*.key"} {
		if !strings.Contains(string(body), want) {
			t.Errorf(".gitignore does not hold %q", want)
		}
	}
}
