package config

import (
	"strings"
	"testing"
	"time"
)

func oneYear() Duration { return Duration{Duration: mustDuration("1y"), raw: "1y"} }

func baseMesh() Mesh {
	m := Mesh{
		Schema:    1,
		Name:      "homelab",
		CIDR:      "10.42.0.0/24",
		Interface: "meshy-homela-a3",
		Nebula:    Nebula{Version: "v1.12.0"},
		CA: CA{
			Duration: Duration{Duration: mustDuration("20y"), raw: "20y"},
			RotateAt: "80%",
			Trust:    []string{"ca-1"},
			Active:   "ca-1",
		},
		Lighthouses: []Lighthouse{{Node: "core-1", Port: 4242}},
		Certificates: Certificates{
			Server:      oneYear(),
			User:        oneYear(),
			RenewBefore: Duration{Duration: mustDuration("60d"), raw: "60d"},
		},
		Nodes: []Node{
			{
				Name:       "core-1",
				Kind:       KindServer,
				Groups:     []string{"core"},
				IP:         "10.42.0.1",
				SSH:        &SSH{User: "meshy", Host: "203.0.113.10", Port: 22},
				PublicAddr: "203.0.113.10:4242",
			},
			{
				Name:   "web-1",
				Kind:   KindServer,
				Groups: []string{"web"},
				IP:     "10.42.0.2",
				SSH:    &SSH{User: "meshy", Host: "203.0.113.11", Port: 22},
			},
			{
				Name:     "horo-laptop",
				Kind:     KindUser,
				Groups:   []string{"user", "admin"},
				IP:       "10.42.0.50",
				Platform: "darwin/arm64",
			},
		},
		Rules: []Rule{
			{From: "user", To: "web-1", Ports: []string{"443/tcp"}},
			{From: "admin", To: "any", Ports: []string{"22/tcp", "any/icmp"}},
		},
	}
	return m
}

func TestValidateAcceptsGoodMesh(t *testing.T) {
	mesh := baseMesh()
	mesh.Defaults()
	if problems := mesh.Validate(); len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Mesh)
		want   string
	}{
		{
			name:   "schema too new",
			mutate: func(m *Mesh) { m.Schema = 2 },
			want:   "schema is 2",
		},
		{
			name:   "interface too long",
			mutate: func(m *Mesh) { m.Interface = "meshy-homelab-abcdef" },
			want:   "at most 15 characters",
		},
		{
			name:   "duplicate ip",
			mutate: func(m *Mesh) { m.Nodes[1].IP = m.Nodes[0].IP },
			want:   "share ip",
		},
		{
			name:   "ip outside cidr",
			mutate: func(m *Mesh) { m.Nodes[1].IP = "10.99.0.2" },
			want:   "outside cidr",
		},
		{
			name:   "network address",
			mutate: func(m *Mesh) { m.Nodes[1].IP = "10.42.0.0" },
			want:   "network or broadcast",
		},
		{
			name:   "user node with ssh",
			mutate: func(m *Mesh) { m.Nodes[2].SSH = &SSH{User: "x", Host: "y", Port: 22} },
			want:   "must not set ssh",
		},
		{
			name:   "user node without platform",
			mutate: func(m *Mesh) { m.Nodes[2].Platform = "" },
			want:   "needs platform",
		},
		{
			name:   "server node without ssh",
			mutate: func(m *Mesh) { m.Nodes[1].SSH = nil },
			want:   "needs an ssh block",
		},
		{
			name:   "lighthouse is not a node",
			mutate: func(m *Mesh) { m.Lighthouses[0].Node = "ghost" },
			want:   "is not a node",
		},
		{
			name:   "lighthouse without address",
			mutate: func(m *Mesh) { m.Nodes[0].PublicAddr = "" },
			want:   "needs public_addr",
		},
		{
			name:   "lighthouse is a user node",
			mutate: func(m *Mesh) { m.Lighthouses[0].Node = "horo-laptop" },
			want:   "must be a server node",
		},
		{
			name:   "rule from unknown group",
			mutate: func(m *Mesh) { m.Rules[0].From = "ghosts" },
			want:   "is not a node or a group",
		},
		{
			name:   "rule with bad port",
			mutate: func(m *Mesh) { m.Rules[0].Ports = []string{"443"} },
			want:   "use a form such as 443/tcp",
		},
		{
			name:   "rule without ports",
			mutate: func(m *Mesh) { m.Rules[0].Ports = nil },
			want:   "has no ports",
		},
		{
			name:   "renew window longer than lifetime",
			mutate: func(m *Mesh) { m.Certificates.RenewBefore = Duration{Duration: mustDuration("2y"), raw: "2y"} },
			want:   "must be shorter than the shortest certificate lifetime",
		},
		{
			name:   "ca shorter than certificates",
			mutate: func(m *Mesh) { m.CA.Duration = Duration{Duration: mustDuration("90d"), raw: "90d"} },
			want:   "must outlast the longest certificate lifetime",
		},
		{
			name:   "active ca not trusted",
			mutate: func(m *Mesh) { m.CA.Active = "ca-9" },
			want:   "is not in ca.trust",
		},
		{
			name:   "duplicate node name",
			mutate: func(m *Mesh) { m.Nodes[1].Name = "core-1" },
			want:   "is named twice",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mesh := baseMesh()
			test.mutate(&mesh)
			problems := mesh.Validate()
			if len(problems) == 0 {
				t.Fatalf("expected a problem that mentions %q, got none", test.want)
			}
			for _, problem := range problems {
				if strings.Contains(problem, test.want) {
					return
				}
			}
			t.Fatalf("expected a problem that mentions %q, got %v", test.want, problems)
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"20y", 20 * 365 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
		{"60d", 60 * 24 * time.Hour},
		{"12h", 12 * time.Hour},
		{"30m", 30 * time.Minute},
		{"90s", 90 * time.Second},
	}
	for _, test := range tests {
		got, err := ParseDuration(test.in)
		if err != nil {
			t.Fatalf("ParseDuration(%q) failed: %v", test.in, err)
		}
		if got != test.want {
			t.Fatalf("ParseDuration(%q) = %v, want %v", test.in, got, test.want)
		}
	}
}

func TestParseDurationRejectsBadValues(t *testing.T) {
	for _, in := range []string{"", "10", "d", "-1d", "0d", "abc"} {
		if _, err := ParseDuration(in); err == nil {
			t.Fatalf("ParseDuration(%q) should fail", in)
		}
	}
}

func TestGenerateInterface(t *testing.T) {
	if got := GenerateInterface("core"); got != "meshy-core" {
		t.Fatalf("short name = %q", got)
	}
	long := GenerateInterface("homelab")
	if len(long) > 15 {
		t.Fatalf("interface %q is longer than 15 characters", long)
	}
	if again := GenerateInterface("homelab"); again != long {
		t.Fatalf("interface name is not stable: %q then %q", long, again)
	}
	if other := GenerateInterface("homelab2"); other == long {
		t.Fatalf("different names share interface %q", long)
	}
}
