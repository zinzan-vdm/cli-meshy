package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zinzan-vdm/cli-meshy/internal/bootstrap"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

func cmdBootstrap(args []string) int {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	name := ""
	// The usage names the node first, and the flag package stops at the first
	// positional argument, so the rest of the line is parsed after it.
	if rest := flags.Args(); len(rest) > 0 {
		name = rest[0]
		if len(rest) > 1 {
			if err := flags.Parse(rest[1:]); err != nil {
				return 2
			}
		}
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "meshy bootstrap: name one node, for example: meshy bootstrap web-1")
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy bootstrap: %v\n", err)
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

	for _, n := range mesh.Nodes {
		if n.Name != name {
			continue
		}
		// The script is the whole deliverable, so a refusal prints nothing on
		// stdout and the maintainer never pipes a half answer into sudo.
		body, err := bootstrap.Script(*mesh, n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Print(body)
		return 0
	}

	fmt.Fprintf(os.Stderr, "meshy bootstrap: %s is not a node in %s\n", name, config.FileName)
	return 2
}
