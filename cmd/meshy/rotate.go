package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/apply"
	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/revoke"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func cmdCARotate(args []string) int {
	flags := flag.NewFlagSet("ca rotate", flag.ContinueOnError)
	phase := flags.Int("phase", 0, "phase of the rotation to run")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	switch *phase {
	case 1:
		return rotatePhase1(*meshDir)
	case 0:
		fmt.Fprintln(os.Stderr, "meshy ca rotate: name a phase with --phase 1")
		return 2
	default:
		fmt.Fprintf(os.Stderr, "meshy ca rotate: phase %d comes in a later change\n", *phase)
		return 2
	}
}

// rotateRun is the state that phase 1 shares between the nodes: the open vault,
// the pinned tool, and the blocklist that every node already holds.
type rotateRun struct {
	*certifier
	dir   string
	block []byte
}

// rotatePhase1 creates the next authority, adds it to ca.trust, and applies the
// trust bundle to every node. It issues no certificate, so a node that the run
// does not reach keeps its old certificate and keeps working.
func rotatePhase1(meshDir string) int {
	mesh, err := config.Load(meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 2
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}
	if len(mesh.CA.Trust) >= 2 {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: ca.trust already holds two authorities")
		return 1
	}

	knownHosts := os.Getenv("MESHY_KNOWN_HOSTS")
	if knownHosts == "" {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: MESHY_KNOWN_HOSTS is not set, so a host key cannot be checked")
		return 2
	}

	run, code := newRotateRun(*mesh, meshDir)
	if code != 0 {
		return code
	}

	// The name comes from ca.trust, so a rotation reads the same every time. A
	// node that is not reached keeps working, and phase 2 reads the same name.
	name := ca.Next(mesh.CA.Trust)

	binary, err := run.tool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 2
	}
	authority, err := ca.Init(context.Background(), ca.Options{
		Binary:   binary,
		Name:     name,
		Networks: mesh.CIDR,
		Lifetime: mesh.CA.Duration.Duration,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}

	// The authority is sealed before the mesh file names it, so mesh.yml never
	// trusts an authority that the vault cannot supply.
	vault.SetCertificate(run.record, name, authority.Cert, authority.Key)
	run.record[name+".issued"] = []byte(authority.Issued.Format("2006-01-02T15:04:05Z"))
	run.changed = true
	if err := vault.Write(meshDir, run.dir, run.record); err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	if _, err := config.AddTrust(meshDir, name); err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}

	after, err := config.Load(meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	bundle, err := vault.TrustBundle(run.record, after.CA.Trust)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}

	report := run.apply(context.Background(), *after, bundle, knownHosts)

	fmt.Printf("authority %s is sealed in %s\n", name, vault.FileName)
	fmt.Printf("mesh.yml now trusts %s\n", strings.Join(after.CA.Trust, " and "))
	fmt.Print(formatRotate(report))
	fmt.Println("next: meshy ca rotate --phase 2")

	if report.Failed > 0 {
		return 1
	}
	return 0
}

// newRotateRun opens the vault and reads the blocklist that the nodes already
// hold, so a run that writes ca.crt does not blank the blocklist beside it.
func newRotateRun(mesh config.Mesh, meshDir string) (*rotateRun, int) {
	dir := filepath.Join(meshDir, vault.DirName)
	record, err := vault.Read(dir)
	if err != nil {
		if errors.Is(err, vault.ErrLocked) {
			fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", vault.ErrLocked)
			return nil, 1
		}
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return nil, 1
	}

	run := &rotateRun{
		certifier: &certifier{mesh: &mesh, meshDir: meshDir, record: record, now: time.Now()},
		dir:       dir,
	}
	if len(revoke.Names(record, mesh)) == 0 {
		run.block = revoke.Text(record, nil)
		return run, 0
	}
	binary, err := run.tool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return nil, 2
	}
	revoked, err := revoke.List(context.Background(), binary, record, mesh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return nil, 1
	}
	run.block = revoke.Text(record, revoked)
	return run, 0
}

// apply writes the trust bundle to every server node and reads the authority
// list back. A node that fails is reported and the run continues, because a node
// that is not reached still trusts the old authority.
func (r *rotateRun) apply(ctx context.Context, mesh config.Mesh, bundle []byte, knownHosts string) apply.Report {
	report := apply.Report{Mesh: mesh.Name, Nodes: []apply.NodeReport{}}
	for _, target := range mesh.Nodes {
		if target.Kind != config.KindServer {
			continue
		}
		report.Add(r.node(ctx, mesh, target, bundle, knownHosts))
	}
	return report
}

func (r *rotateRun) node(ctx context.Context, mesh config.Mesh, target config.Node, bundle []byte, knownHosts string) apply.NodeReport {
	report := apply.NodeReport{Name: target.Name}

	cert, key, found := vault.Certificate(r.record, target.Name)
	if !found {
		report.Error = fmt.Sprintf("the vault holds no certificate for %s", target.Name)
		return report
	}

	rendered, err := nebula.Render(mesh, target)
	if err != nil {
		report.Error = err.Error()
		return report
	}

	session, err := dialSession(target, knownHosts)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	if c, ok := session.(interface{ Close() error }); ok {
		defer c.Close()
	}

	result, err := apply.Run(ctx, mesh, target, apply.Files{
		Config:    rendered,
		Authority: bundle,
		Cert:      cert,
		Key:       key,
		Blocklist: r.block,
	}, session)
	report.Writes = result.Writes
	report.Action = result.Action
	if err != nil {
		report.Error = reason(target, err)
		return report
	}

	certs, err := node.ReadAuthorities(ctx, mesh, target, session)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	if missing := missingAuthority(certs, mesh.CA.Trust); missing != "" {
		report.Error = fmt.Sprintf("the trust bundle on the node does not hold %s", missing)
	}
	return report
}

// missingAuthority returns the first authority that the mesh trusts and the
// node's bundle does not hold, or an empty string when the node holds every one.
func missingAuthority(certs []node.Cert, trust []string) string {
	onNode := make(map[string]bool, len(certs))
	for _, cert := range certs {
		onNode[cert.Name] = true
	}
	for _, name := range trust {
		if !onNode[name] {
			return name
		}
	}
	return ""
}

func formatRotate(report apply.Report) string {
	var out strings.Builder
	for _, n := range report.Nodes {
		switch {
		case n.Error != "":
			fmt.Fprintf(&out, "%s: failed: %s\n", n.Name, n.Error)
		case len(n.Writes) == 0:
			fmt.Fprintf(&out, "%s: unchanged\n", n.Name)
		default:
			fmt.Fprintf(&out, "%s: %s\n", n.Name, applied(n))
		}
	}
	return out.String()
}
