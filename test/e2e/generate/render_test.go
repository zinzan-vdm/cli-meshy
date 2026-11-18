package main

import (
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// TestRenderCoversTheMesh runs the renderer that a node would use on the mesh
// file that the e2e run mounts. It proves the four configs exist and carry the
// rules that the four assertions depend on.
func TestRenderCoversTheMesh(t *testing.T) {
	mesh, err := config.Load("..")
	if err != nil {
		t.Fatalf("load the e2e mesh: %v", err)
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		t.Fatalf("the e2e mesh has problems: %v", problems)
	}

	configs, err := render(*mesh)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(configs) != 4 {
		t.Fatalf("got %d configs, want one per node and one more for node-a: %d", len(configs), len(configs))
	}

	for _, name := range []string{"lighthouse", "node-a", "node-b", allowKey} {
		body, ok := configs[name]
		if !ok {
			t.Fatalf("no config for %s", name)
		}
		if !strings.Contains(string(body), "/etc/nebula/meshy-lab/") {
			t.Errorf("config for %s does not name the mounted directory:\n%s", name, body)
		}
	}

	lighthouse := string(configs["lighthouse"])
	if !strings.Contains(lighthouse, "port: 4242") {
		t.Errorf("the lighthouse does not listen on 4242:\n%s", lighthouse)
	}
	if strings.Contains(lighthouse, "static_host_map") {
		t.Errorf("the lighthouse holds a static host map:\n%s", lighthouse)
	}

	nodeA := string(configs["node-a"])
	if !strings.Contains(nodeA, "port: 8080") {
		t.Errorf("node-a does not allow 8080:\n%s", nodeA)
	}
	if strings.Contains(nodeA, "9090") {
		t.Errorf("node-a already allows 9090, so assertion 3 would fail:\n%s", nodeA)
	}
	if !strings.Contains(nodeA, "proto: icmp") {
		t.Errorf("node-a does not allow icmp, so assertion 1 would fail:\n%s", nodeA)
	}
	if !strings.Contains(nodeA, "group: user") {
		t.Errorf("node-a does not allow the group that node-b holds:\n%s", nodeA)
	}

	opened := string(configs[allowKey])
	if !strings.Contains(opened, "port: 9090") {
		t.Errorf("the second config does not allow 9090:\n%s", opened)
	}

	nodeB := string(configs["node-b"])
	if !strings.Contains(nodeB, "lighthouse:4242") {
		t.Errorf("node-b has no route to the lighthouse:\n%s", nodeB)
	}
	if !strings.Contains(nodeB, "cert: /etc/nebula/meshy-lab/node-b.crt") {
		t.Errorf("node-b does not read its own certificate:\n%s", nodeB)
	}
}
