package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/apply"
	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/revoke"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// applyRun carries the state that one invocation shares between nodes: the open
// vault, the tool that signs, and the certificates that the mesh no longer
// names.
type applyRun struct {
	mesh    *config.Mesh
	meshDir string
	dir     string
	record  vault.Record
	binary  string
	now     time.Time
	block   []byte
	revoked []revoke.Node
	changed bool
}

func cmdApply(args []string) int {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	name := flags.String("node", "", "apply to one node")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	dryRun := flags.Bool("dry-run", false, "print the plan and write nothing")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "meshy apply: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	if *dryRun {
		return cmdPlan(planArgs(*meshDir, *asJSON))
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

	targets, code := applyTargets(*mesh, *name)
	if code != 0 {
		return code
	}

	knownHosts := os.Getenv("MESHY_KNOWN_HOSTS")
	if knownHosts == "" {
		fmt.Fprintln(os.Stderr, "meshy apply: MESHY_KNOWN_HOSTS is not set, so a host key cannot be checked")
		return 2
	}

	run, code := newApplyRun(*mesh, *meshDir)
	if code != 0 {
		return code
	}

	report := run.run(context.Background(), targets, knownHosts)

	if *asJSON {
		body, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
			return 2
		}
		os.Stdout.Write(append(body, '\n'))
	} else {
		fmt.Print(formatApply(report))
	}

	if !report.Clean() {
		return 1
	}
	return 0
}

// planArgs builds the arguments that make apply --dry-run print the plan.
func planArgs(meshDir string, asJSON bool) []string {
	args := []string{"--mesh", meshDir}
	if asJSON {
		args = append(args, "--json")
	}
	return args
}

// newApplyRun opens the vault and reads what every node will need. The tool that
// signs is looked up when a run needs it, so a run that renews nothing and
// revokes nothing does not need it.
func newApplyRun(mesh config.Mesh, meshDir string) (*applyRun, int) {
	dir := filepath.Join(meshDir, vault.DirName)
	record, err := vault.Read(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy apply: %v\n", err)
		return nil, 2
	}

	run := &applyRun{mesh: &mesh, meshDir: meshDir, dir: dir, record: record, now: time.Now()}
	return run, 0
}

// tool returns the pinned nebula-cert. A run that issues or revokes needs it.
func (r *applyRun) tool() (string, error) {
	if r.binary != "" {
		return r.binary, nil
	}
	binary, err := ca.Find(r.meshDir, r.mesh.Nebula.Version)
	if err != nil {
		return "", err
	}
	r.binary = binary
	return binary, nil
}

// run makes every target hold the mesh, blocks a certificate that the mesh no
// longer names, and reports one line for each node. A node that fails does not
// stop the others.
func (r *applyRun) run(ctx context.Context, targets []config.Node, knownHosts string) apply.Report {
	report := apply.Report{Mesh: r.mesh.Name, Nodes: []apply.NodeReport{}}

	revoked, blocklist, err := r.blocklist(ctx)
	if err != nil {
		report.Add(apply.NodeReport{Name: r.mesh.Name, Error: err.Error()})
		return report
	}
	r.block = blocklist
	r.revoked = revoked
	for _, n := range revoked {
		report.Revoked = append(report.Revoked, apply.Revocation{Name: n.Name, Fingerprint: n.Fingerprint})
	}

	for _, target := range targets {
		report.Add(r.node(ctx, target, knownHosts))
	}

	// The certificates leave the vault only when every node took the blocklist.
	// A run that failed somewhere keeps the record, so the next run retries.
	if len(r.revoked) > 0 && report.Clean() {
		for _, n := range r.revoked {
			vault.DeleteCertificate(r.record, n.Name)
			vault.SetRevoked(r.record, n.Name, n.Fingerprint)
			r.changed = true
		}
	}

	if r.changed {
		if err := vault.Write(r.meshDir, r.dir, r.record); err != nil {
			report.Add(apply.NodeReport{Name: r.mesh.Name, Error: err.Error()})
		}
	}

	return report
}

// blocklist returns the certificates that the mesh no longer names and the
// bytes that every node must hold. Nothing is deleted here, because the nodes
// have to take the blocklist first.
func (r *applyRun) blocklist(ctx context.Context) ([]revoke.Node, []byte, error) {
	if len(revoke.Names(r.record, *r.mesh)) == 0 {
		return nil, revoke.Text(r.record, nil), nil
	}

	binary, err := r.tool()
	if err != nil {
		return nil, nil, err
	}
	revoked, err := revoke.List(ctx, binary, r.record, *r.mesh)
	if err != nil {
		return nil, nil, err
	}
	return revoked, revoke.Text(r.record, revoked), nil
}

// node makes one node hold the files of the mesh. It renews a certificate that
// is missing or inside the window before it writes, so one run causes one
// reload.
func (r *applyRun) node(ctx context.Context, target config.Node, knownHosts string) apply.NodeReport {
	report := apply.NodeReport{Name: target.Name}

	session, err := dialSession(target, knownHosts)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	if c, ok := session.(interface{ Close() error }); ok {
		defer c.Close()
	}

	cert, key, renewed, err := r.certificate(ctx, target)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	report.Renewed = renewed

	files, err := r.files(target, cert, key)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}

	result, err := apply.Run(ctx, *r.mesh, target, files, session)
	report.Writes = result.Writes
	report.Action = result.Action
	if err != nil {
		report.Error = reason(target, err)
	}
	return report
}

// certificate returns the certificate and the key that the node must hold. It
// signs a new certificate when the vault holds none or when the one it holds is
// inside the renewal window, and it keeps the address and the groups, so the
// node needs a reload and not a restart.
func (r *applyRun) certificate(ctx context.Context, target config.Node) (cert, key []byte, renewed bool, err error) {
	binary, err := r.tool()
	if err != nil {
		return nil, nil, false, err
	}

	if current, currentKey, ok := vault.Certificate(r.record, target.Name); ok && !due(ctx, binary, current, *r.mesh, r.now) {
		return current, currentKey, false, nil
	}

	networks, err := meshNetworks(*r.mesh, target)
	if err != nil {
		return nil, nil, false, err
	}
	signed, err := issue.Sign(ctx, issue.Options{
		Binary:    binary,
		Authority: r.authority(),
		Request: issue.Request{
			Name:     target.Name,
			Networks: networks,
			Groups:   target.Groups,
			Lifetime: lifetime(*r.mesh, target),
		},
	})
	if err != nil {
		return nil, nil, false, err
	}

	vault.SetCertificate(r.record, target.Name, signed.Cert, signed.Key)
	r.changed = true
	return signed.Cert, signed.Key, true, nil
}

// authority returns the key pair that signs. The vault must hold it, because a
// run that renews a certificate needs the authority key.
func (r *applyRun) authority() issue.Authority {
	cert, key, _ := vault.Certificate(r.record, r.mesh.CA.Active)
	return issue.Authority{Cert: cert, Key: key}
}

// due reports whether the certificate that the vault holds needs to be replaced.
// The vault copy is the certificate that the run writes, so its remaining life
// decides the renewal.
func due(ctx context.Context, binary string, cert []byte, mesh config.Mesh, now time.Time) bool {
	printed, err := issue.Print(ctx, binary, cert)
	if err != nil {
		return true
	}
	notAfter, err := issue.Expiry(printed)
	if err != nil {
		return true
	}
	return issue.Due(notAfter, mesh.Certificates.RenewBefore.Duration, now, false)
}

// files reads the material that the node must hold.
func (r *applyRun) files(target config.Node, cert, key []byte) (apply.Files, error) {
	authority, _, found := vault.Certificate(r.record, r.mesh.CA.Active)
	if !found {
		return apply.Files{}, fmt.Errorf("the vault holds no certificate for authority %s", r.mesh.CA.Active)
	}

	rendered, err := nebula.Render(*r.mesh, target)
	if err != nil {
		return apply.Files{}, err
	}

	return apply.Files{
		Config:    rendered,
		Authority: authority,
		Cert:      cert,
		Key:       key,
		Blocklist: r.block,
	}, nil
}

// reason drops the node name that a package already wrote, so the report names
// the node once.
func reason(target config.Node, err error) string {
	return strings.TrimPrefix(err.Error(), "node "+target.Name+": ")
}

func formatApply(report apply.Report) string {
	var out strings.Builder
	for _, revoked := range report.Revoked {
		fmt.Fprintf(&out, "revoked %s (%s)\n", revoked.Name, revoked.Fingerprint)
	}
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

// applied writes one line for a node that changed, in the order the run did the
// work.
func applied(n apply.NodeReport) string {
	parts := make([]string, 0, 3)
	if n.Renewed {
		parts = append(parts, "renewed")
	}
	parts = append(parts, "wrote "+strings.Join(n.Writes, " "))
	switch n.Action {
	case apply.ActionReload:
		parts = append(parts, "reloaded")
	case apply.ActionRestart:
		parts = append(parts, "started")
	}
	return strings.Join(parts, ", ")
}

// applyTargets returns the nodes that a run covers. A user node is never read
// or written over SSH, so a run that names one stops and names the command that
// builds its bundle.
func applyTargets(mesh config.Mesh, name string) ([]config.Node, int) {
	if name == "" {
		targets := make([]config.Node, 0, len(mesh.Nodes))
		for _, n := range mesh.Nodes {
			if n.Kind == config.KindServer {
				targets = append(targets, n)
			}
		}
		return targets, 0
	}

	target, found := applyTarget(mesh, name)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy apply: node %q is not in %s\n", name, config.FileName)
		return nil, 2
	}
	if target.Kind != config.KindServer {
		fmt.Fprintf(os.Stderr, "meshy apply: %s is a user node, run meshy bundle\n", target.Name)
		return nil, 1
	}
	return []config.Node{target}, 0
}

// applyTarget returns the node that name identifies.
func applyTarget(mesh config.Mesh, name string) (config.Node, bool) {
	for _, n := range mesh.Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return config.Node{}, false
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
