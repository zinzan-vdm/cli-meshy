package main

import (
	"errors"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/plan"
)

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
