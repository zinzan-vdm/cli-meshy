package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/plan"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx"
)

func cmdPlan(args []string) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print the plan as JSON")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy plan: %v\n", err)
		return 2
	}

	problems := mesh.Validate()
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}

	knownHosts := os.Getenv("MESHY_KNOWN_HOSTS")
	if knownHosts == "" {
		fmt.Fprintln(os.Stderr, "meshy plan: MESHY_KNOWN_HOSTS is not set, so a host key cannot be checked")
		return 2
	}

	report := plan.Compute(plan.Input{
		Mesh:     *mesh,
		Readings: readNodes(*mesh, knownHosts),
		Now:      time.Now(),
	})

	if *asJSON {
		body, err := formatJSON(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "meshy plan: %v\n", err)
			return 2
		}
		os.Stdout.Write(body)
	} else {
		fmt.Print(formatText(report))
	}

	return exitCode(report)
}

// exitCode keeps the three outcomes apart: a match, a difference, and a plan
// that could not be computed.
func exitCode(r plan.Report) int {
	if r.Failed() {
		return 2
	}
	if !r.Clean() {
		return 1
	}
	return 0
}

func readNodes(mesh config.Mesh, knownHosts string) []plan.Reading {
	states, errs := node.ReadAll(context.Background(), mesh, func(n config.Node) (node.Executor, error) {
		cfg, err := sshx.FromNode(n, knownHosts)
		if err != nil {
			return nil, err
		}
		return sshx.Dial(cfg)
	})

	readings := make([]plan.Reading, 0, len(states)+len(errs))
	for _, state := range states {
		readings = append(readings, plan.Reading{Name: state.Name, State: state})
	}
	for _, err := range errs {
		readings = append(readings, plan.Reading{Name: readNode(err), Err: err})
	}
	return readings
}

// readNode returns the node that a read error names. Every error from
// node.ReadAll starts with the node name, which is the contract of a read.
func readNode(err error) string {
	rest := strings.TrimPrefix(err.Error(), "node ")
	if i := strings.Index(rest, ":"); i > 0 {
		return rest[:i]
	}
	return rest
}

func formatText(report plan.Report) string {
	nameWidth, kindWidth := 0, 0
	for _, n := range report.Nodes {
		nameWidth = max(nameWidth, len(n.Name))
		for _, change := range n.Changes {
			kindWidth = max(kindWidth, len(string(change.Kind)))
		}
		for _, s := range statusLines(n) {
			kindWidth = max(kindWidth, len(s.kind))
		}
	}
	nameWidth += 2
	kindWidth += 2

	var out strings.Builder
	for _, n := range report.Nodes {
		for _, change := range n.Changes {
			fmt.Fprintf(&out, "%-*s%-*s%s\n", nameWidth, n.Name, kindWidth, change.Kind, change.Detail)
		}
		for _, s := range statusLines(n) {
			if s.detail == "" {
				fmt.Fprintf(&out, "%-*s%s\n", nameWidth, n.Name, s.kind)
				continue
			}
			fmt.Fprintf(&out, "%-*s%-*s%s\n", nameWidth, n.Name, kindWidth, s.kind, s.detail)
		}
	}
	return out.String()
}

type statusLine struct {
	kind   string
	detail string
}

func statusLines(n plan.Node) []statusLine {
	switch {
	case n.Error != "":
		return []statusLine{{kind: "error", detail: n.Error}}
	case n.Unread:
		return []statusLine{{kind: "unread"}}
	case len(n.Changes) == 0:
		return []statusLine{{kind: "ok"}}
	}
	return nil
}

func formatJSON(report plan.Report) ([]byte, error) {
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format the plan: %w", err)
	}
	return append(body, '\n'), nil
}
