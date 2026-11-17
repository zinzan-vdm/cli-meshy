package nebula

import (
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

func TestExpand(t *testing.T) {
	tests := []struct {
		name  string
		rules []config.Rule
		node  string
		want  []Rule
	}{
		{
			name:  "group rule to one node",
			rules: []config.Rule{{From: "user", To: "web-1", Ports: []string{"443/tcp"}}},
			node:  "web-1",
			want:  []Rule{{Port: NumberPort(443), Proto: "tcp", Group: "user"}},
		},
		{
			name:  "group rule does not reach another node",
			rules: []config.Rule{{From: "user", To: "web-1", Ports: []string{"443/tcp"}}},
			node:  "horo-laptop",
			want:  []Rule{},
		},
		{
			name:  "host rule from a node name",
			rules: []config.Rule{{From: "core-1", To: "web-1", Ports: []string{"9100/tcp"}}},
			node:  "web-1",
			want:  []Rule{{Port: NumberPort(9100), Proto: "tcp", Host: "core-1"}},
		},
		{
			name:  "any destination reaches every node",
			rules: []config.Rule{{From: "admin", To: "any", Ports: []string{"22/tcp"}}},
			node:  "web-1",
			want:  []Rule{{Port: NumberPort(22), Proto: "tcp", Group: "admin"}},
		},
		{
			name:  "port range",
			rules: []config.Rule{{From: "user", To: "web-1", Ports: []string{"8000-8100/tcp"}}},
			node:  "web-1",
			want:  []Rule{{Port: RangePort("8000-8100"), Proto: "tcp", Group: "user"}},
		},
		{
			name:  "icmp has no port",
			rules: []config.Rule{{From: "user", To: "web-1", Ports: []string{"any/icmp"}}},
			node:  "web-1",
			want:  []Rule{{Port: Port{}, Proto: "icmp", Group: "user"}},
		},
		{
			name: "duplicates collapse",
			rules: []config.Rule{
				{From: "user", To: "web-1", Ports: []string{"443/tcp"}},
				{From: "user", To: "any", Ports: []string{"443/tcp"}},
			},
			node: "web-1",
			want: []Rule{{Port: NumberPort(443), Proto: "tcp", Group: "user"}},
		},
		{
			name:  "no rules means deny all",
			rules: nil,
			node:  "web-1",
			want:  []Rule{},
		},
		{
			name: "two rules sort by source",
			rules: []config.Rule{
				{From: "user", To: "web-1", Ports: []string{"443/tcp"}},
				{From: "core-1", To: "web-1", Ports: []string{"9100/tcp"}},
			},
			node: "web-1",
			want: []Rule{
				{Port: NumberPort(9100), Proto: "tcp", Host: "core-1"},
				{Port: NumberPort(443), Proto: "tcp", Group: "user"},
			},
		},
		{
			name:  "any source",
			rules: []config.Rule{{From: "any", To: "web-1", Ports: []string{"80/tcp"}}},
			node:  "web-1",
			want:  []Rule{{Port: NumberPort(80), Proto: "tcp", Host: "any"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mesh := fixture()
			mesh.Rules = test.rules
			got, err := Expand(mesh, nodeByName(t, mesh, test.node))
			if err != nil {
				t.Fatalf("Expand failed: %v", err)
			}
			if len(got) != len(test.want) {
				t.Fatalf("got %d rules, want %d: %v", len(got), len(test.want), got)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("rule %d = %+v, want %+v", i, got[i], test.want[i])
				}
			}
		})
	}
}

func TestExpandRefusesBadRules(t *testing.T) {
	t.Run("unknown group", func(t *testing.T) {
		mesh := fixture()
		mesh.Rules = []config.Rule{{From: "ghosts", To: "web-1", Ports: []string{"443/tcp"}}}
		if _, err := Expand(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for an unknown group")
		}
	})

	t.Run("port with no protocol", func(t *testing.T) {
		mesh := fixture()
		mesh.Rules = []config.Rule{{From: "user", To: "web-1", Ports: []string{"443"}}}
		if _, err := Expand(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for a port with no protocol")
		}
	})

	t.Run("port out of range", func(t *testing.T) {
		mesh := fixture()
		mesh.Rules = []config.Rule{{From: "user", To: "web-1", Ports: []string{"70000/tcp"}}}
		if _, err := Expand(mesh, nodeByName(t, mesh, "web-1")); err == nil {
			t.Fatal("expected an error for a port out of range")
		}
	})
}

func TestExpandIsStable(t *testing.T) {
	mesh := fixture()
	node := nodeByName(t, mesh, "web-1")

	first, err := Expand(mesh, node)
	if err != nil {
		t.Fatalf("first expand: %v", err)
	}
	second, err := Expand(mesh, node)
	if err != nil {
		t.Fatalf("second expand: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("lengths differ: %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("entry %d differs: %+v and %+v", i, first[i], second[i])
		}
	}
}
