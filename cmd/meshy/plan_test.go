package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/plan"
)

// rotationReport is a plan that read one authority of the length mesh.yml
// writes as 20y.
func rotationReport(used float64, due bool, next string) plan.Report {
	return plan.Report{
		Mesh: "homelab",
		Rotation: &plan.Rotation{
			Authority: "ca-1",
			NotBefore: "2026-01-01T00:00:00Z",
			NotAfter:  "2046-01-01T00:00:00Z",
			Used:      used,
			RotateAt:  80,
			Due:       due,
			Next:      next,
		},
		Nodes: []plan.Node{{Name: "core-1"}},
	}
}

func TestFormatRotationPrintsTheAgeAboveTheNodes(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2046, 1, 1, 0, 0, 0, 0, time.UTC)
	due := start.Add(time.Duration(float64(end.Sub(start)) * 0.8))

	got := formatRotation(rotationReport(0.4, false, "phase 1"))
	want := "mesh homelab\n" +
		"  authority  ca-1 (0% of 20y)\n" +
		"  due        no\n" +
		"  next       phase 1 from " + due.Format("2006-01-02") + "\n\n"
	if got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatRotationPrintsTheCommandWhenTheAuthorityIsDue(t *testing.T) {
	got := formatRotation(rotationReport(81.2, true, "phase 1"))
	if !strings.Contains(got, "  due        yes\n") {
		t.Errorf("text =\n%s\nwant the due line to say yes", got)
	}
	if !strings.Contains(got, "next       phase 1 now: meshy ca rotate --phase 1\n") {
		t.Errorf("text =\n%s\nwant the command that starts the rotation", got)
	}
}

func TestFormatRotationNamesThePhaseInProgress(t *testing.T) {
	got := formatRotation(rotationReport(4, false, "phase 2"))
	if !strings.Contains(got, "next       phase 2\n") {
		t.Errorf("text =\n%s\nwant phase 2", got)
	}
}

func TestFormatRotationPrintsNothingWithoutAnAuthority(t *testing.T) {
	if got := formatRotation(plan.Report{Mesh: "homelab"}); got != "" {
		t.Errorf("text = %q, want nothing", got)
	}
}

func TestFormatJSONCarriesTheRotation(t *testing.T) {
	body, err := formatJSON(rotationReport(0.4, false, "phase 1"))
	if err != nil {
		t.Fatalf("formatJSON: %v", err)
	}
	for _, want := range []string{
		`"rotation": {`,
		`"authority": "ca-1",`,
		`"not_before": "2026-01-01T00:00:00Z",`,
		`"not_after": "2046-01-01T00:00:00Z",`,
		`"used": 0.4,`,
		`"rotate_at": 80,`,
		`"due": false,`,
		`"next": "phase 1"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("json does not hold %q:\n%s", want, body)
		}
	}
}

func TestFormatJSONLeavesTheRotationOut(t *testing.T) {
	body, err := formatJSON(plan.Report{Mesh: "homelab", Nodes: []plan.Node{}})
	if err != nil {
		t.Fatalf("formatJSON: %v", err)
	}
	if strings.Contains(string(body), "rotation") {
		t.Errorf("json names a rotation that the plan did not read:\n%s", body)
	}
}

func TestFormatTextPrintsOneLinePerDifference(t *testing.T) {
	report := plan.Report{
		Mesh: "homelab",
		Nodes: []plan.Node{
			{Name: "core-1", Changes: []plan.Change{
				{Kind: plan.KindBinary, Detail: "runs v1.11.0, pinned v1.12.0"},
			}},
			{Name: "db-1"},
			{Name: "web-1", Changes: []plan.Change{
				{Kind: plan.KindCertificate, Detail: "expires in 12d, renews"},
				{Kind: plan.KindConfig, Detail: "differs"},
			}},
		},
		Changes: 3,
	}

	want := `core-1  binary       runs v1.11.0, pinned v1.12.0
db-1    ok
web-1   certificate  expires in 12d, renews
web-1   config       differs
`
	if got := formatText(report); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatTextMarksUnreadAndFailedNodes(t *testing.T) {
	report := plan.Report{
		Mesh: "homelab",
		Nodes: []plan.Node{
			{Name: "core-1", Error: "read failed: connection refused"},
			{Name: "laptop", Unread: true},
			{Name: "web-1", Changes: []plan.Change{{Kind: plan.KindConfig, Detail: "differs"}}},
		},
		Changes: 1,
	}

	want := `core-1  error   read failed: connection refused
laptop  unread
web-1   config  differs
`
	if got := formatText(report); got != want {
		t.Errorf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatJSONKeepsTheShape(t *testing.T) {
	report := plan.Report{
		Mesh: "homelab",
		Nodes: []plan.Node{
			{Name: "web-1", Changes: []plan.Change{
				{Kind: plan.KindCertificate, Detail: "expires in 12d"},
			}},
		},
		Changes: 1,
	}

	want := `{
  "mesh": "homelab",
  "nodes": [
    {
      "name": "web-1",
      "changes": [
        {
          "kind": "certificate",
          "detail": "expires in 12d"
        }
      ]
    }
  ],
  "changes": 1
}
`
	body, err := formatJSON(report)
	if err != nil {
		t.Fatalf("formatJSON: %v", err)
	}
	if string(body) != want {
		t.Errorf("json =\n%s\nwant\n%s", body, want)
	}
}

func TestFormatJSONPrintsAnEmptyNodeList(t *testing.T) {
	body, err := formatJSON(plan.Report{Mesh: "homelab", Nodes: []plan.Node{}})
	if err != nil {
		t.Fatalf("formatJSON: %v", err)
	}
	want := `{
  "mesh": "homelab",
  "nodes": [],
  "changes": 0
}
`
	if string(body) != want {
		t.Errorf("json =\n%s\nwant\n%s", body, want)
	}
}

func TestReadNodeNamesTheNodeInAReadError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("node web-1: nebula -version: connection refused"), "web-1"},
		{errors.New("node core-1: dial 203.0.113.10:22: i/o timeout"), "core-1"},
		{errors.New("no node in this error"), "no node in this error"},
	}
	for _, c := range cases {
		if got := readNode(c.err); got != c.want {
			t.Errorf("readNode(%q) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestExitCodeKeepsTheThreeOutcomesApart(t *testing.T) {
	clean := plan.Report{Mesh: "homelab", Nodes: []plan.Node{{Name: "core-1"}}}
	if got := exitCode(clean); got != 0 {
		t.Errorf("clean plan exit = %d, want 0", got)
	}

	differ := plan.Report{Mesh: "homelab", Changes: 1, Nodes: []plan.Node{{
		Name:    "core-1",
		Changes: []plan.Change{{Kind: plan.KindConfig, Detail: "differs"}},
	}}}
	if got := exitCode(differ); got != 1 {
		t.Errorf("changed plan exit = %d, want 1", got)
	}

	// A node that cannot be read is not a difference, so it gets its own code.
	failed := plan.Report{Mesh: "homelab", Nodes: []plan.Node{{Name: "core-1", Error: "read failed"}}}
	if got := exitCode(failed); got != 2 {
		t.Errorf("failed plan exit = %d, want 2", got)
	}
}
