package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

const issueDateFormat = "2006-01-02"

func cmdIssue(args []string) int {
	flags := flag.NewFlagSet("issue", flag.ContinueOnError)
	force := flags.Bool("force", false, "sign a new certificate even when the stored one is valid")
	all := flags.Bool("all", false, "issue a certificate for every node in mesh.yml")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	// flag.Parse stops at the first name it sees, and the synopsis puts the node
	// before the flags, so the arguments after the node are read as flags.
	name := flags.Arg(0)
	rest := flags.Args()
	if len(rest) > 0 {
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "meshy issue: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	switch {
	case *all && name != "":
		fmt.Fprintln(os.Stderr, "meshy issue: name a node or use --all, not both")
		return 2
	case !*all && name == "":
		fmt.Fprintln(os.Stderr, "meshy issue: name a node or use --all")
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy issue: %v\n", err)
		return 2
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}

	targets, code := issueTargets(mesh, name, *all)
	if code != 0 {
		return code
	}

	dir := filepath.Join(*meshDir, vault.DirName)
	record, err := vault.Read(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy issue: %v\n", err)
		if errors.Is(err, vault.ErrLocked) {
			return 1
		}
		return 2
	}

	authorityCert, authorityKey, found := vault.Certificate(record, mesh.CA.Active)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy issue: the vault holds no key for authority %s\n", mesh.CA.Active)
		return 2
	}

	binary, err := ca.Find(*meshDir, mesh.Nebula.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy issue: %v\n", err)
		return 2
	}

	run := &issueRun{
		mesh:      mesh,
		record:    record,
		binary:    binary,
		authority: issue.Authority{Cert: authorityCert, Key: authorityKey},
		force:     *force,
		now:       time.Now(),
	}

	changed := false
	for _, node := range targets {
		signed, code := run.node(context.Background(), node)
		if code != 0 {
			return code
		}
		changed = changed || signed
	}

	if changed {
		if err := vault.Write(*meshDir, dir, record); err != nil {
			fmt.Fprintf(os.Stderr, "meshy issue: %v\n", err)
			return 2
		}
	}

	return 0
}

// issueRun carries the state that one invocation shares between nodes.
type issueRun struct {
	mesh      *config.Mesh
	record    vault.Record
	binary    string
	authority issue.Authority
	force     bool
	now       time.Time
}

// node issues a certificate for one node when the node needs one. It reports
// whether it changed the record.
func (r *issueRun) node(ctx context.Context, n config.Node) (bool, int) {
	existing, _, stored := vault.Certificate(r.record, n.Name)

	var notAfter time.Time
	if stored && !r.force {
		printed, err := issue.Print(ctx, r.binary, existing)
		if err != nil {
			fmt.Fprintf(os.Stderr, "meshy issue: node %s: %v\n", n.Name, err)
			return false, 2
		}
		notAfter, err = issue.Expiry(printed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "meshy issue: node %s: %v\n", n.Name, err)
			return false, 2
		}
	}

	if !issue.Due(notAfter, r.mesh.Certificates.RenewBefore.Duration, r.now, r.force) {
		fmt.Printf("%s: already valid until %s\n", n.Name, notAfter.Format(issueDateFormat))
		return false, 0
	}

	networks, err := r.networks(n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy issue: %v\n", err)
		return false, 2
	}

	signed, err := issue.Sign(ctx, issue.Options{
		Binary:    r.binary,
		Authority: r.authority,
		Request: issue.Request{
			Name:     n.Name,
			Networks: networks,
			Groups:   n.Groups,
			Lifetime: r.lifetime(n),
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy issue: node %s: %v\n", n.Name, err)
		return false, 2
	}

	vault.SetCertificate(r.record, n.Name, signed.Cert, signed.Key)
	fmt.Printf("%s: issued, expires %s, fingerprint %s\n", n.Name, signed.NotAfter.Format(issueDateFormat), signed.Fingerprint)
	return true, 0
}

// networks writes the node address with the mesh prefix, because nebula-cert
// takes the address of a node as a network.
func (r *issueRun) networks(n config.Node) (string, error) {
	prefix, err := netip.ParsePrefix(r.mesh.CIDR)
	if err != nil {
		return "", fmt.Errorf("cidr %q is not readable: %w", r.mesh.CIDR, err)
	}
	return fmt.Sprintf("%s/%d", n.IP, prefix.Bits()), nil
}

// lifetime returns the certificate lifetime for the kind of the node.
func (r *issueRun) lifetime(n config.Node) time.Duration {
	if n.Kind == config.KindUser {
		return r.mesh.Certificates.User.Duration
	}
	return r.mesh.Certificates.Server.Duration
}

// issueTargets returns the nodes that a run covers.
func issueTargets(mesh *config.Mesh, name string, all bool) ([]config.Node, int) {
	if all {
		return mesh.Nodes, 0
	}
	for _, node := range mesh.Nodes {
		if node.Name == name {
			return []config.Node{node}, 0
		}
	}
	fmt.Fprintf(os.Stderr, "meshy issue: node %q is not in %s\n", name, config.FileName)
	return nil, 1
}
