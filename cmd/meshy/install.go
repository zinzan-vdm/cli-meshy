package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/install"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/release"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func cmdInstall(args []string) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	name := flags.String("node", "", "install on one node")
	source := flags.String("source", "", "directory that holds the archives, instead of a download")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "meshy install: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy install: %v\n", err)
		return 2
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}

	targets, code := installTargets(*mesh, *name)
	if code != 0 {
		return code
	}

	knownHosts := os.Getenv("MESHY_KNOWN_HOSTS")
	if knownHosts == "" {
		fmt.Fprintln(os.Stderr, "meshy install: MESHY_KNOWN_HOSTS is not set, so a host key cannot be checked")
		return 2
	}

	cache := platformCache(*mesh, *meshDir, *source)
	results := install.Run(context.Background(), *mesh, targets, installOpen(knownHosts), cache)

	code = 0
	for _, result := range results {
		code |= reportInstall(*mesh, result)
	}
	return code
}

// installTargets is every server node, or the one that --node names.
func installTargets(mesh config.Mesh, name string) ([]config.Node, int) {
	if name != "" {
		target, found := applyTarget(mesh, name)
		if !found {
			fmt.Fprintf(os.Stderr, "meshy install: node %q is not in %s\n", name, config.FileName)
			return nil, 2
		}
		if target.Kind != config.KindServer {
			fmt.Fprintf(os.Stderr, "meshy install: %s is a user node, run meshy bundle\n", target.Name)
			return nil, 1
		}
		return []config.Node{target}, 0
	}

	var targets []config.Node
	for _, n := range mesh.Nodes {
		if n.Kind == config.KindServer {
			targets = append(targets, n)
		}
	}
	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "meshy install: mesh %q holds no server node\n", mesh.Name)
		return nil, 2
	}
	return targets, 0
}

// platformCache keeps one binary per platform in .meshy, which is where the
// bundle reads it from.
func platformCache(mesh config.Mesh, meshDir, source string) func(context.Context, install.Platform) (string, error) {
	var mu sync.Mutex
	kept := map[string]string{}

	return func(ctx context.Context, p install.Platform) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		if path, ok := kept[p.String()]; ok {
			return path, nil
		}

		dir := filepath.Join(meshDir, vault.DirName, "bin", mesh.Nebula.Version, p.OS+"-"+p.Arch)
		var path string
		var err error
		if source != "" {
			path, err = release.Local(source, mesh.Nebula.Version, p.OS, p.Arch, dir)
		} else {
			path, err = release.Fetch(ctx, http.DefaultClient, mesh.Nebula.Version, p.OS, p.Arch, dir)
		}
		if err != nil {
			return "", err
		}
		kept[p.String()] = path
		return path, nil
	}
}

// installOpen is the connection that one node needs. A test replaces it to run
// the command with no node.
var installOpen = func(knownHosts string) func(config.Node) (install.Session, error) {
	return func(n config.Node) (install.Session, error) {
		cfg, err := sshx.FromNode(n, knownHosts)
		if err != nil {
			return nil, err
		}
		return sshx.Dial(cfg)
	}
}

// reportInstall prints one node and returns the part of the exit code it owns.
func reportInstall(mesh config.Mesh, r install.Result) int {
	if r.Err != nil {
		fmt.Fprintf(os.Stderr, "meshy install: %v\n", r.Err)
		return 1
	}
	if !r.Moved {
		fmt.Printf("%s: %s\n", r.Node, mesh.Nebula.Version)
		return 0
	}
	from := r.From
	if from == "" {
		from = "nothing"
	}
	fmt.Printf("%s: %s, was %s, restarted %s\n", r.Node, mesh.Nebula.Version, from, node.Unit(mesh.Name))
	return 0
}
