package plan

import (
	"strings"
	"testing"
)

func TestComputeReportsARevokedNodeAsItsOwnRow(t *testing.T) {
	m := testMesh()
	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Revoked:  []Revoked{{Name: "old-1", Fingerprint: "4f3a"}},
		Now:      now,
	})

	row := nodeRow(t, report, "old-1")
	if len(row.Changes) != 1 {
		t.Fatalf("the revoked node holds %d changes, want 1: %+v", len(row.Changes), row)
	}
	if row.Changes[0].Kind != KindRevoked {
		t.Errorf("kind = %q, want revoked", row.Changes[0].Kind)
	}
	if !strings.Contains(row.Changes[0].Detail, "4f3a") {
		t.Errorf("the detail does not name the fingerprint: %q", row.Changes[0].Detail)
	}
	if report.Clean() {
		t.Error("a plan that revokes a node is clean")
	}
	if report.Changes != 1 {
		t.Errorf("changes = %d, want 1", report.Changes)
	}
}

func TestComputeKeepsTheBlocklistOutOfTheConfigComparison(t *testing.T) {
	m := testMesh()
	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Revoked:  []Revoked{{Name: "old-1", Fingerprint: "4f3a"}},
		Now:      now,
	})

	for _, name := range []string{"core-1", "web-1"} {
		row := nodeRow(t, report, name)
		if len(row.Changes) != 0 || row.Error != "" {
			t.Errorf("node %s holds a config change: %+v", name, row)
		}
	}
}

func TestComputeIgnoresARevokedNameThatTheFileNames(t *testing.T) {
	m := testMesh()
	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Revoked:  []Revoked{{Name: "web-1", Fingerprint: "4f3a"}},
		Now:      now,
	})

	if report.Changes != 0 {
		t.Errorf("changes = %d, want 0: %+v", report.Changes, report.Nodes)
	}
}

func TestComputeSortsARevokedRowWithTheNodes(t *testing.T) {
	m := testMesh()
	report := Compute(Input{
		Mesh:     m,
		Readings: healthyReadings(t, m),
		Revoked:  []Revoked{{Name: "old-1", Fingerprint: "4f3a"}},
		Now:      now,
	})

	names := make([]string, 0, len(report.Nodes))
	for _, n := range report.Nodes {
		names = append(names, n.Name)
	}
	want := []string{"core-1", "laptop", "old-1", "web-1"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("the plan names %v, want %v", names, want)
	}
}
