// Command generate writes the configs and certificates that the e2e mesh runs.
// It renders every config through internal/nebula, so the run proves the
// renderer that a node would use.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
)

// allowPort is the port that the fourth assertion opens. The mesh file allows
// 8080 and this command renders one more config for node-a with 9090 added.
const allowPort = "9090"

// allowNode is the node whose config is rendered twice. It is the destination
// of every assertion.
const allowNode = "node-a"

func main() {
	meshDir := flag.String("mesh", "test/e2e", "directory that holds mesh.yml")
	binary := flag.String("binary", "", "path to the pinned nebula-cert")
	out := flag.String("out", "test/e2e/generated", "directory for the generated files")
	flag.Parse()

	if *binary == "" {
		fmt.Fprintln(os.Stderr, "generate: -binary names the pinned nebula-cert")
		os.Exit(2)
	}
	if err := run(*meshDir, *binary, *out); err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
}

func run(meshDir, binary, out string) error {
	mesh, err := config.Load(meshDir)
	if err != nil {
		return err
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return fmt.Errorf("mesh %q has %d problems", mesh.Name, len(problems))
	}

	configs, err := render(*mesh)
	if err != nil {
		return err
	}

	authority, err := ca.Init(context.Background(), ca.Options{
		Binary:   binary,
		Name:     mesh.CA.Active,
		Networks: mesh.CIDR,
		Lifetime: mesh.CA.Duration.Duration,
	})
	if err != nil {
		return err
	}

	for _, node := range mesh.Nodes {
		signed, err := sign(binary, *mesh, authority, node)
		if err != nil {
			return err
		}
		files := map[string]file{
			"config.yml":       {body: configs[node.Name], mode: 0o600},
			"ca.crt":           {body: authority.Cert, mode: 0o644},
			node.Name + ".crt": {body: signed.Cert, mode: 0o644},
			node.Name + ".key": {body: signed.Key, mode: 0o600},
			"blocklist.txt":    {body: nil, mode: 0o644},
		}
		if node.Name == allowNode {
			files["config-allow.yml"] = file{body: configs[allowKey], mode: 0o600}
		}
		if err := writeAll(filepath.Join(out, node.Name), files); err != nil {
			return err
		}
	}

	fmt.Printf("generate: %d nodes, %d configs, one authority\n", len(mesh.Nodes), len(configs))
	return nil
}

// allowKey names the second config of node-a. The harness swaps it in and sends
// SIGHUP.
const allowKey = allowNode + "-allow"

// render returns one config for each node, plus the node-a config that holds
// the extra rule. It reads no file and opens no connection.
func render(mesh config.Mesh) (map[string][]byte, error) {
	configs := make(map[string][]byte, len(mesh.Nodes)+1)
	for _, node := range mesh.Nodes {
		body, err := nebula.Render(mesh, node)
		if err != nil {
			return nil, err
		}
		configs[node.Name] = body
	}

	from, err := groupOf(mesh, "node-b")
	if err != nil {
		return nil, err
	}
	to, err := groupOf(mesh, allowNode)
	if err != nil {
		return nil, err
	}

	opened := mesh
	opened.Rules = append(append([]config.Rule(nil), mesh.Rules...), config.Rule{
		From:  from,
		To:    to,
		Ports: []string{allowPort + "/tcp"},
	})

	target, err := nodeOf(mesh, allowNode)
	if err != nil {
		return nil, err
	}
	body, err := nebula.Render(opened, target)
	if err != nil {
		return nil, err
	}
	configs[allowKey] = body
	return configs, nil
}

func groupOf(mesh config.Mesh, name string) (string, error) {
	node, err := nodeOf(mesh, name)
	if err != nil {
		return "", err
	}
	if len(node.Groups) == 0 {
		return "", fmt.Errorf("node %q holds no group", name)
	}
	return node.Groups[0], nil
}

func nodeOf(mesh config.Mesh, name string) (config.Node, error) {
	for _, node := range mesh.Nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return config.Node{}, fmt.Errorf("node %q is not in %s", name, config.FileName)
}

// sign keeps the time that the certificate is valid and the network that it
// covers in one place, so both come from the mesh file.
func sign(binary string, mesh config.Mesh, authority ca.Authority, node config.Node) (issue.Certificate, error) {
	prefix, err := netip.ParsePrefix(mesh.CIDR)
	if err != nil {
		return issue.Certificate{}, fmt.Errorf("cidr %q is not readable: %w", mesh.CIDR, err)
	}
	lifetime := mesh.Certificates.Server.Duration
	if node.Kind == config.KindUser {
		lifetime = mesh.Certificates.User.Duration
	}
	return issue.Sign(context.Background(), issue.Options{
		Binary:    binary,
		Authority: issue.Authority{Cert: authority.Cert, Key: authority.Key},
		Request: issue.Request{
			Name:     node.Name,
			Networks: fmt.Sprintf("%s/%d", node.IP, prefix.Bits()),
			Groups:   node.Groups,
			Lifetime: lifetime,
		},
	})
}

type file struct {
	body []byte
	mode os.FileMode
}

func writeAll(dir string, files map[string]file) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, held := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, held.body, held.mode); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
