package nebula

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func year() config.Duration { return config.Duration{Duration: 365 * 24 * time.Hour} }

// fixture returns a three node mesh: one lighthouse, one server node, and one
// user node. Two groups, two rules.
func fixture() config.Mesh {
	return config.Mesh{
		Schema:    1,
		Name:      "homelab",
		CIDR:      "10.42.0.0/24",
		Interface: "meshy-homelab",
		Nebula:    config.Nebula{Version: "v1.12.0"},
		CA: config.CA{
			Duration: config.Duration{Duration: 20 * 365 * 24 * time.Hour},
			RotateAt: "80%",
			Trust:    []string{"ca-1"},
			Active:   "ca-1",
		},
		Lighthouses: []config.Lighthouse{{Node: "core-1", Port: 4242}},
		Certificates: config.Certificates{
			Server:      year(),
			User:        year(),
			RenewBefore: config.Duration{Duration: 60 * 24 * time.Hour},
		},
		Nodes: []config.Node{
			{
				Name:       "core-1",
				Kind:       config.KindServer,
				Groups:     []string{"core"},
				IP:         "10.42.0.1",
				SSH:        &config.SSH{User: "meshy", Host: "203.0.113.10", Port: 22},
				PublicAddr: "203.0.113.10:4242",
			},
			{
				Name:   "web-1",
				Kind:   config.KindServer,
				Groups: []string{"web"},
				IP:     "10.42.0.2",
				SSH:    &config.SSH{User: "meshy", Host: "203.0.113.11", Port: 22},
			},
			{
				Name:     "horo-laptop",
				Kind:     config.KindUser,
				Groups:   []string{"user", "admin"},
				IP:       "10.42.0.50",
				Platform: "darwin/arm64",
			},
		},
		Rules: []config.Rule{
			{From: "user", To: "web-1", Ports: []string{"443/tcp"}},
			{From: "admin", To: "any", Ports: []string{"22/tcp", "any/icmp"}},
		},
	}
}

func nodeByName(t *testing.T, mesh config.Mesh, name string) config.Node {
	t.Helper()
	node, found := findNode(mesh, name)
	if !found {
		t.Fatalf("fixture has no node %q", name)
	}
	return node
}

func TestRenderGolden(t *testing.T) {
	cases := []struct {
		name string
		mesh func() config.Mesh
		node string
	}{
		{name: "lighthouse", mesh: fixture, node: "core-1"},
		{name: "server", mesh: fixture, node: "web-1"},
		{name: "user", mesh: fixture, node: "horo-laptop"},
		{name: "two-lighthouses", mesh: twoLighthouses, node: "web-1"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mesh := test.mesh()
			got, err := Render(mesh, nodeByName(t, mesh, test.node))
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			golden := filepath.Join("testdata", test.name+".yml")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatalf("create testdata: %v", err)
				}
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write %s: %v", golden, err)
				}
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read %s: %v (run with -update to create it)", golden, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s does not match the golden file.\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
			}
		})
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	mesh := fixture()
	node := nodeByName(t, mesh, "web-1")

	first, err := Render(mesh, node)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	second, err := Render(mesh, node)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two renders of one input differ")
	}
}

func TestRenderRefusesBadMeshes(t *testing.T) {
	t.Run("lighthouse without a public address", func(t *testing.T) {
		mesh := fixture()
		mesh.Nodes[0].PublicAddr = ""
		if _, err := Render(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for a lighthouse with no public_addr")
		}
	})

	t.Run("mesh without a lighthouse", func(t *testing.T) {
		mesh := fixture()
		mesh.Lighthouses = nil
		if _, err := Render(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for a mesh with no lighthouse")
		}
	})

	t.Run("lighthouse that is not a node", func(t *testing.T) {
		mesh := fixture()
		mesh.Lighthouses[0].Node = "ghost"
		if _, err := Render(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for a lighthouse that is not a node")
		}
	})
}

// twoLighthouses returns the fixture with a second lighthouse.
func twoLighthouses() config.Mesh {
	mesh := fixture()
	mesh.Nodes = append(mesh.Nodes, config.Node{
		Name:       "core-2",
		Kind:       config.KindServer,
		Groups:     []string{"core"},
		IP:         "10.42.0.3",
		SSH:        &config.SSH{User: "meshy", Host: "203.0.113.12", Port: 22},
		PublicAddr: "203.0.113.12:4242",
	})
	mesh.Lighthouses = append(mesh.Lighthouses, config.Lighthouse{Node: "core-2", Port: 4242})
	return mesh
}
