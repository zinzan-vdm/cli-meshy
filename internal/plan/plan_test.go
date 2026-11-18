package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

var now = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

func testMesh() config.Mesh {
	m := config.Mesh{
		Name:        "homelab",
		CIDR:        "10.42.0.0/24",
		Interface:   "meshy0",
		Nebula:      config.Nebula{Version: "v1.12.0"},
		Lighthouses: []config.Lighthouse{{Node: "core-1", Port: 4242}},
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1", PublicAddr: "203.0.113.10:4242", SSH: &config.SSH{User: "meshy", Host: "203.0.113.10"}},
			{Name: "web-1", Kind: config.KindServer, Groups: []string{"web"}, IP: "10.42.0.2", SSH: &config.SSH{User: "meshy", Host: "203.0.113.11"}},
			{Name: "laptop", Kind: config.KindUser, Groups: []string{"people"}, IP: "10.42.0.50", Platform: "darwin/arm64"},
		},
		Rules: []config.Rule{{From: "people", To: "web-1", Ports: []string{"443/tcp"}}},
	}
	m.Defaults()
	return m
}

func meshNode(t *testing.T, m config.Mesh, name string) config.Node {
	t.Helper()
	for _, n := range m.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node %q in the test mesh", name)
	return config.Node{}
}

func configHash(t *testing.T, m config.Mesh, n config.Node) string {
	t.Helper()
	body, err := nebula.Render(m, n)
	if err != nil {
		t.Fatalf("Render(%s): %v", n.Name, err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func stateFor(t *testing.T, m config.Mesh, name string, expires time.Time) node.State {
	t.Helper()
	n := meshNode(t, m, name)
	return node.State{
		Name:       n.Name,
		Version:    strings.TrimPrefix(m.Nebula.Version, "v"),
		Service:    "active",
		Interface:  true,
		ConfigHash: configHash(t, m, n),
		Cert: node.Cert{
			Present:     true,
			Name:        n.Name,
			Fingerprint: "aa" + n.Name,
			Groups:      n.Groups,
			Addresses:   []string{n.IP},
			NotAfter:    expires.Format(time.RFC3339),
		},
		Authorities: []node.Cert{{Present: true, Name: "ca-1", IsCA: true}},
	}
}

func healthyReadings(t *testing.T, m config.Mesh) []Reading {
	t.Helper()
	return []Reading{
		{Name: "core-1", State: stateFor(t, m, "core-1", now.Add(200*24*time.Hour))},
		{Name: "web-1", State: stateFor(t, m, "web-1", now.Add(200*24*time.Hour))},
	}
}

func nodeRow(t *testing.T, report Report, name string) Node {
	t.Helper()
	for _, n := range report.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node %q in the plan: %+v", name, report.Nodes)
	return Node{}
}

func TestComputeReportsNoChangeForAMatchingMesh(t *testing.T) {
	m := testMesh()
	report := Compute(Input{Mesh: m, Readings: healthyReadings(t, m), Now: now})

	if report.Mesh != "homelab" {
		t.Errorf("mesh = %q", report.Mesh)
	}
	if report.Changes != 0 {
		t.Errorf("changes = %d, want 0: %+v", report.Changes, report.Nodes)
	}
	if !report.Clean() {
		t.Error("a matching mesh is not clean")
	}
	if len(report.Nodes) != 3 {
		t.Fatalf("plan holds %d nodes, want 3", len(report.Nodes))
	}
	for _, n := range report.Nodes {
		if len(n.Changes) != 0 || n.Error != "" {
			t.Errorf("node %s holds a fact: %+v", n.Name, n)
		}
	}
}

func TestComputeReportsEachDifference(t *testing.T) {
	m := testMesh()

	cases := []struct {
		name   string
		change func(st *node.State)
		kind   Kind
		detail string
	}{
		{
			name:   "binary runs another release",
			change: func(st *node.State) { st.Version = "1.11.0" },
			kind:   KindBinary,
			detail: "runs v1.11.0, pinned v1.12.0",
		},
		{
			name:   "service is down",
			change: func(st *node.State) { st.Service = "inactive" },
			kind:   KindService,
			detail: "unit nebula@meshy-homelab is inactive",
		},
		{
			name:   "service state is unreadable",
			change: func(st *node.State) { st.Service = "" },
			kind:   KindService,
			detail: "unit nebula@meshy-homelab is unknown",
		},
		{
			name:   "interface is absent",
			change: func(st *node.State) { st.Interface = false },
			kind:   KindInterface,
			detail: "device meshy0 is absent",
		},
		{
			name:   "config differs",
			change: func(st *node.State) { st.ConfigHash = strings.Repeat("0", 64) },
			kind:   KindConfig,
			detail: "differs",
		},
		{
			name:   "certificate is missing",
			change: func(st *node.State) { st.Cert = node.Cert{} },
			kind:   KindCertificate,
			detail: "no certificate",
		},
		{
			name:   "certificate holds other groups",
			change: func(st *node.State) { st.Cert.Groups = []string{"db"} },
			kind:   KindCertificate,
			detail: "groups db, mesh web",
		},
		{
			name:   "certificate holds no groups",
			change: func(st *node.State) { st.Cert.Groups = nil },
			kind:   KindCertificate,
			detail: "groups none, mesh web",
		},
		{
			name:   "certificate holds another address",
			change: func(st *node.State) { st.Cert.Addresses = []string{"10.42.0.9"} },
			kind:   KindCertificate,
			detail: "address 10.42.0.9, mesh 10.42.0.2",
		},
		{
			name:   "certificate names another node",
			change: func(st *node.State) { st.Cert.Name = "web-2" },
			kind:   KindCertificate,
			detail: "certificate names web-2, mesh names web-1",
		},
		{
			name:   "certificate is inside the renewal window",
			change: func(st *node.State) { st.Cert.NotAfter = now.Add(12 * 24 * time.Hour).Format(time.RFC3339) },
			kind:   KindCertificate,
			detail: "expires in 12d, renews",
		},
		{
			name:   "certificate expired",
			change: func(st *node.State) { st.Cert.NotAfter = now.Add(-72 * time.Hour).Format(time.RFC3339) },
			kind:   KindCertificate,
			detail: "expired 3d ago",
		},
		{
			name:   "certificate expiry is not readable",
			change: func(st *node.State) { st.Cert.NotAfter = "soon" },
			kind:   KindCertificate,
			detail: `expiry "soon" is not readable`,
		},
		{
			name: "authority is not trusted",
			change: func(st *node.State) {
				st.Authorities = append(st.Authorities, node.Cert{Present: true, Name: "ca-2", IsCA: true})
			},
			kind:   KindCertificate,
			detail: "authority ca-2 is not trusted",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := stateFor(t, m, "web-1", now.Add(200*24*time.Hour))
			c.change(&st)

			report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})
			row := nodeRow(t, report, "web-1")

			if len(row.Changes) != 1 {
				t.Fatalf("changes = %+v, want one", row.Changes)
			}
			if row.Changes[0].Kind != c.kind || row.Changes[0].Detail != c.detail {
				t.Errorf("change = %+v, want %s %q", row.Changes[0], c.kind, c.detail)
			}
			if row.Error != "" {
				t.Errorf("error = %q", row.Error)
			}
			if report.Changes != 1 || report.Clean() {
				t.Errorf("changes = %d, clean = %v", report.Changes, report.Clean())
			}
		})
	}
}

func TestComputeReportsAnAuthorityThatTheNodeDoesNotTrust(t *testing.T) {
	m := testMesh()
	m.CA.Trust = []string{"ca-1", "ca-2"}

	st := stateFor(t, m, "web-1", now.Add(200*24*time.Hour))
	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})
	row := nodeRow(t, report, "web-1")

	if len(row.Changes) != 1 || row.Changes[0].Detail != "the node does not trust ca-2" {
		t.Fatalf("changes = %+v, want the missing authority", row.Changes)
	}
}

func TestComputeReportsNoChangeWhenTheNodeTrustsEveryAuthority(t *testing.T) {
	m := testMesh()
	m.CA.Trust = []string{"ca-1", "ca-2"}

	st := stateFor(t, m, "web-1", now.Add(200*24*time.Hour))
	st.Authorities = []node.Cert{
		{Present: true, Name: "ca-1", IsCA: true},
		{Present: true, Name: "ca-2", IsCA: true},
	}
	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})
	if row := nodeRow(t, report, "web-1"); len(row.Changes) != 0 {
		t.Errorf("changes = %+v, want none", row.Changes)
	}
}

func TestComputeRenewsOnlyInsideTheWindow(t *testing.T) {
	m := testMesh()
	window := m.Certificates.RenewBefore.Duration

	edge := stateFor(t, m, "web-1", now.Add(window))
	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: edge}}, Now: now})
	if report.Changes != 0 {
		t.Errorf("a certificate that holds exactly the window reported %+v", nodeRow(t, report, "web-1").Changes)
	}

	inside := stateFor(t, m, "web-1", now.Add(window-time.Hour))
	report = Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: inside}}, Now: now})
	if report.Changes != 1 {
		t.Errorf("a certificate inside the window reported %+v", nodeRow(t, report, "web-1").Changes)
	}
}

func TestComputeAcceptsAnAddressWithTheMeshPrefix(t *testing.T) {
	m := testMesh()
	st := stateFor(t, m, "web-1", now.Add(200*24*time.Hour))
	st.Cert.Addresses = []string{"10.42.0.2/24"}

	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})
	if report.Changes != 0 {
		t.Errorf("a certificate with the mesh prefix reported %+v", nodeRow(t, report, "web-1").Changes)
	}
}

func TestComputeComparesTheConfigByContent(t *testing.T) {
	m := testMesh()
	st := stateFor(t, m, "web-1", now.Add(200*24*time.Hour))
	st.ConfigHash = configHash(t, m, meshNode(t, m, "core-1"))

	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})
	row := nodeRow(t, report, "web-1")
	if len(row.Changes) != 1 || row.Changes[0].Kind != KindConfig {
		t.Fatalf("changes = %+v, want one config change", row.Changes)
	}
}

func TestComputeSortsNodesAndKinds(t *testing.T) {
	m := testMesh()
	st := stateFor(t, m, "web-1", now.Add(12*24*time.Hour))
	st.Version = "1.11.0"
	st.Service = "failed"
	st.Interface = false
	st.ConfigHash = strings.Repeat("0", 64)

	report := Compute(Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now})

	names := make([]string, len(report.Nodes))
	for i, n := range report.Nodes {
		names[i] = n.Name
	}
	if want := []string{"core-1", "laptop", "web-1"}; !reflect.DeepEqual(names, want) {
		t.Errorf("node order = %v, want %v", names, want)
	}

	row := nodeRow(t, report, "web-1")
	kinds := make([]Kind, len(row.Changes))
	for i, c := range row.Changes {
		kinds[i] = c.Kind
	}
	want := []Kind{KindBinary, KindCertificate, KindConfig, KindInterface, KindService}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("kind order = %v, want %v", kinds, want)
	}
}

func TestComputeMarksAUserNodeUnread(t *testing.T) {
	m := testMesh()
	report := Compute(Input{Mesh: m, Readings: healthyReadings(t, m), Now: now})

	row := nodeRow(t, report, "laptop")
	if !row.Unread {
		t.Error("a user node is not marked unread")
	}
	if len(row.Changes) != 0 || row.Error != "" {
		t.Errorf("a user node with no vault record reported %+v", row)
	}
}

func TestComputeReportsAUserCertificateFromTheVault(t *testing.T) {
	m := testMesh()
	record := node.Cert{
		Present:     true,
		Name:        "laptop",
		Fingerprint: "bb-laptop",
		Groups:      []string{"people"},
		Addresses:   []string{"10.42.0.50"},
		NotAfter:    now.Add(9 * 24 * time.Hour).Format(time.RFC3339),
	}

	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Certs:    map[string]node.Cert{"laptop": record},
		Now:      now,
	})

	row := nodeRow(t, report, "laptop")
	if !row.Unread {
		t.Error("a user node is not marked unread")
	}
	if len(row.Changes) != 1 {
		t.Fatalf("changes = %+v, want one", row.Changes)
	}
	if row.Changes[0].Kind != KindCertificate || row.Changes[0].Detail != "expires in 9d, renews" {
		t.Errorf("change = %+v", row.Changes[0])
	}
}

func TestComputeComparesAServerCertificateWithTheVault(t *testing.T) {
	m := testMesh()
	record := node.Cert{Present: true, Name: "web-1", Fingerprint: "cc-other"}

	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Certs:    map[string]node.Cert{"web-1": record},
		Now:      now,
	})

	row := nodeRow(t, report, "web-1")
	if len(row.Changes) != 1 || row.Changes[0].Detail != "differs from the vault" {
		t.Errorf("changes = %+v", row.Changes)
	}
}

func TestComputeReportsAFailedRead(t *testing.T) {
	m := testMesh()
	readings := []Reading{
		{Name: "core-1", State: stateFor(t, m, "core-1", now.Add(200*24*time.Hour))},
		{Name: "web-1", Err: errors.New("node web-1: nebula -version: connection refused")},
	}

	report := Compute(Input{Mesh: m, Readings: readings, Now: now})
	row := nodeRow(t, report, "web-1")

	if row.Error != "read failed: nebula -version: connection refused" {
		t.Errorf("error = %q", row.Error)
	}
	if len(row.Changes) != 0 {
		t.Errorf("a failed read holds changes: %+v", row.Changes)
	}
	if report.Clean() {
		t.Error("a plan with a failed read is clean")
	}
}

func TestComputeReportsANodeThatWasNotRead(t *testing.T) {
	m := testMesh()
	readings := []Reading{{Name: "core-1", State: stateFor(t, m, "core-1", now.Add(200*24*time.Hour))}}

	report := Compute(Input{Mesh: m, Readings: readings, Now: now})
	row := nodeRow(t, report, "web-1")

	if row.Error != "read failed: the node was not read" {
		t.Errorf("error = %q", row.Error)
	}
	if report.Clean() {
		t.Error("a plan with an unread node is clean")
	}
}

func TestComputeKeepsAReadThatNamesNoNodeInTheFile(t *testing.T) {
	m := testMesh()
	readings := append(healthyReadings(t, m), Reading{Name: "ghost-1", State: node.State{Name: "ghost-1"}})

	report := Compute(Input{Mesh: m, Readings: readings, Now: now})
	row := nodeRow(t, report, "ghost-1")

	if row.Error != "read a node that mesh.yml does not name" {
		t.Errorf("error = %q", row.Error)
	}
	if report.Clean() {
		t.Error("a plan with a node outside the file is clean")
	}
}

func TestComputeIsDeterministic(t *testing.T) {
	m := testMesh()
	st := stateFor(t, m, "web-1", now.Add(5*24*time.Hour))
	st.Version = "1.10.0"
	st.Cert.Groups = []string{"db", "web"}
	in := Input{Mesh: m, Readings: []Reading{{Name: "web-1", State: st}}, Now: now}

	first := Compute(in)
	second := Compute(in)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two runs differ:\n%+v\n%+v", first, second)
	}
}
