package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/apply"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func cmdApply(args []string) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	name := flags.String("node", "", "apply to one node")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "meshy apply: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if *name == "" {
		fmt.Fprintln(os.Stderr, "meshy apply: name one node with --node, for example: meshy apply --node web-1")
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
		return 2
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}

	target, found := applyTarget(*mesh, *name)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy apply: node %q is not in %s\n", *name, config.FileName)
		return 2
	}
	if target.Kind != config.KindServer {
		fmt.Fprintf(os.Stderr, "meshy apply: %s is a user node, run meshy bundle\n", target.Name)
		return 1
	}

	knownHosts := os.Getenv("MESHY_KNOWN_HOSTS")
	if knownHosts == "" {
		fmt.Fprintln(os.Stderr, "meshy apply: MESHY_KNOWN_HOSTS is not set, so a host key cannot be checked")
		return 2
	}

	files, code := applyFiles(*mesh, target, *meshDir)
	if code != 0 {
		return code
	}

	// MESHY-042 replaces this single call with a loop over every node and a
	// report. Everything that one node needs is already here.
	result, code := applyNode(context.Background(), *mesh, target, files, knownHosts)
	if code != 0 {
		return code
	}

	reportOne(target, *mesh, result)
	return 0
}

// applyFiles reads the material that the node must hold. The vault must be open
// and must hold a certificate for the node and for the active authority.
func applyFiles(mesh config.Mesh, n config.Node, meshDir string) (apply.Files, int) {
	record, err := vault.Read(filepath.Join(meshDir, vault.DirName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
		return apply.Files{}, 2
	}

	authority, _, found := vault.Certificate(record, mesh.CA.Active)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy apply: the vault holds no certificate for authority %s\n", mesh.CA.Active)
		return apply.Files{}, 2
	}
	cert, key, found := vault.Certificate(record, n.Name)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy apply: the vault holds no certificate for %s, run meshy issue %s\n", n.Name, n.Name)
		return apply.Files{}, 2
	}

	rendered, err := nebula.Render(mesh, n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
		return apply.Files{}, 2
	}

	return apply.Files{Config: rendered, Authority: authority, Cert: cert, Key: key}, 0
}

func applyNode(ctx context.Context, mesh config.Mesh, n config.Node, files apply.Files, knownHosts string) (apply.Result, int) {
	session, err := dialSession(n, knownHosts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: node %s: %v\n", n.Name, err)
		return apply.Result{}, 1
	}
	if c, ok := session.(interface{ Close() error }); ok {
		defer c.Close()
	}

	result, err := apply.Run(ctx, mesh, n, files, session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
		return result, 1
	}
	return result, 0
}

// dialSession opens the connection that one node needs. A test replaces it to
// run the command with no node.
var dialSession = func(n config.Node, knownHosts string) (apply.Session, error) {
	cfg, err := sshx.FromNode(n, knownHosts)
	if err != nil {
		return nil, err
	}
	return sshx.Dial(cfg)
}

func applyTarget(mesh config.Mesh, name string) (config.Node, bool) {
	for _, n := range mesh.Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return config.Node{}, false
}

func reportOne(n config.Node, mesh config.Mesh, r apply.Result) {
	if len(r.Writes) == 0 {
		fmt.Printf("%s: unchanged\n", n.Name)
		return
	}
	fmt.Printf("%s: wrote %s\n", n.Name, strings.Join(r.Writes, " "))
	switch r.Action {
	case apply.ActionReload:
		fmt.Printf("%s: reloaded %s\n", n.Name, node.Unit(mesh.Name))
	case apply.ActionRestart:
		fmt.Printf("%s: started %s\n", n.Name, node.Unit(mesh.Name))
	}
}
