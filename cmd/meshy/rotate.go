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
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
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
	case 2:
		return rotatePhase2(*meshDir)
	case 3:
		return rotatePhase3(*meshDir)
	case 0:
		fmt.Fprintln(os.Stderr, "meshy ca rotate: name a phase with --phase 1, 2, or 3")
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
	// drop names an authority that a bundle must not hold when the run reads it
	// back. Phase 3 sets it, because the drop is the point of that phase.
	drop string
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
	if r.drop != "" {
		for _, cert := range certs {
			if cert.Name == r.drop {
				report.Error = fmt.Sprintf("the trust bundle on the node still holds %s", r.drop)
			}
		}
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

// rotateAuthority is the authority that signs the certificates of phase 2. The
// fingerprint is the value that the issuer field of a node certificate must
// name, so one read back compares it with one string.
type rotateAuthority struct {
	name        string
	cert        []byte
	key         []byte
	fingerprint string
}

// rotatePhase2 makes the new authority active and replaces the certificate of
// every server node with one that the new authority signed. Every node already
// trusts both authorities, so a node reloads into its new certificate on a live
// tunnel. A node that does not come back stops the phase and the rest keep what
// they hold. See docs/adr/0012.
func rotatePhase2(meshDir string) int {
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
	if len(mesh.CA.Trust) < 2 {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: run phase 1 first")
		return 1
	}

	// The highest trusted authority is the one that phase 1 added. An active
	// authority that already holds that number means the mesh file names it.
	target := ca.NewActive(mesh.CA.Trust, mesh.CA.Active)
	name := target
	if name == "" {
		// The mesh file already names the authority, so this run only finishes
		// the nodes that a stopped phase did not reach.
		name = mesh.CA.Active
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

	cert, key, found := vault.Certificate(run.record, name)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: the vault holds no certificate for authority %s\n", name)
		return 1
	}
	binary, err := run.tool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 2
	}
	printed, err := issue.Print(context.Background(), binary, cert)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	bundle, err := vault.TrustBundle(run.record, mesh.CA.Trust)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	signers, err := authorityFingerprints(context.Background(), binary, run.record, mesh.CA.Trust)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}

	// The file names the authority before the first node is touched, so a run
	// that stops halfway leaves a mesh file that agrees with the nodes it did
	// reach. A second run then finishes the nodes that it did not reach.
	if target != "" {
		if _, err := config.SetActive(meshDir, name); err != nil {
			fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
			return 1
		}
		after, err := config.Load(meshDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
			return 1
		}
		run.mesh = after
		mesh = after
	}

	report, stopped, untouched := run.rotate(context.Background(), *mesh, rotateAuthority{
		name:        name,
		cert:        cert,
		key:         key,
		fingerprint: printed.Fingerprint,
	}, bundle, knownHosts, signers)

	if target != "" {
		fmt.Printf("authority %s now signs new certificates\n", name)
		fmt.Printf("mesh.yml now makes %s active\n", name)
	} else {
		fmt.Printf("authority %s already signs every certificate\n", name)
	}
	fmt.Print(formatRotatePhase2(report, name, untouched))
	if stopped != "" {
		fmt.Printf("phase 2 stopped at %s, run phase 2 again when it answers\n", stopped)
		return 1
	}
	if !report.Changed() {
		fmt.Println("no change")
	}
	fmt.Println("next: meshy ca rotate --phase 3")
	return 0
}

// rotatePhase3 removes the old authority from ca.trust and applies the bundle
// that is left to every node. It refuses while a node still holds a certificate
// that the old authority signed, because that node would lose the trust that it
// needs to reach the rest of the mesh. The old key stays in the vault, because a
// deleted key cannot explain a certificate that survives in a backup.
func rotatePhase3(meshDir string) int {
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
	if len(mesh.CA.Trust) < 2 {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: the rotation is already complete")
		return 1
	}
	// Phase 3 drops the authority that the nodes no longer use. Before phase 2
	// the newest authority holds no certificate, so dropping it would undo
	// phase 1 and leave the mesh with the authority that it is leaving.
	if newest := ca.NewActive(mesh.CA.Trust, mesh.CA.Active); newest != "" {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: run phase 2 first")
		return 1
	}

	old := oldAuthority(mesh.CA)
	if old == "" {
		fmt.Fprintln(os.Stderr, "meshy ca rotate: ca.trust holds no authority that ca.active does not name")
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
	binary, err := run.tool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 2
	}
	signers, err := authorityFingerprints(context.Background(), binary, run.record, mesh.CA.Trust)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	active := trustedFingerprint(signers, mesh.CA.Active)
	if active == "" {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: the vault holds no certificate for authority %s\n", mesh.CA.Active)
		return 1
	}

	// The signing authority of every node is read before the file changes, so a
	// refusal leaves the mesh file and every node exactly as they were.
	issuers, err := run.signingAuthorities(context.Background(), *mesh, knownHosts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	for _, target := range mesh.Nodes {
		if target.Kind != config.KindServer {
			continue
		}
		if strings.EqualFold(issuers[target.Name], active) {
			continue
		}
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %s still holds a certificate from %s\n",
			target.Name, signerOf(signers, issuers[target.Name]))
		return 1
	}

	// The file drops the authority before the bundles change, so a run that
	// stops halfway leaves a mesh file that agrees with the nodes it reached.
	if _, err := config.RemoveTrust(meshDir, old); err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	after, err := config.Load(meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}
	run.mesh = after
	run.drop = old
	mesh = after

	bundle, err := vault.TrustBundle(run.record, mesh.CA.Trust)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca rotate: %v\n", err)
		return 1
	}

	report := run.apply(context.Background(), *mesh, bundle, knownHosts)
	fmt.Printf("mesh.yml no longer trusts %s\n", old)
	fmt.Print(formatRotate(report))
	if !report.Clean() {
		fmt.Println("phase 3 stopped, run phase 3 again when the node answers")
		return 1
	}
	fmt.Printf("%s.key stays in %s\n", old, vault.FileName)
	fmt.Println("the rotation is complete")
	return 0
}

// oldAuthority returns the trusted authority that ca.active does not name. A
// rotation holds two authorities, so the other one is the old one.
func oldAuthority(caState config.CA) string {
	for _, name := range caState.Trust {
		if name != caState.Active {
			return name
		}
	}
	return ""
}

// signingAuthorities reads the issuer of the certificate on every server node.
// The phase refuses on a node that it cannot read, because the run cannot prove
// what that node holds.
func (r *rotateRun) signingAuthorities(ctx context.Context, mesh config.Mesh, knownHosts string) (map[string]string, error) {
	issuers := make(map[string]string, len(mesh.Nodes))
	for _, target := range mesh.Nodes {
		if target.Kind != config.KindServer {
			continue
		}
		session, err := dialSession(target, knownHosts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", target.Name, err)
		}
		held, err := node.ReadCert(ctx, mesh, target, session)
		if c, ok := session.(interface{ Close() error }); ok {
			c.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %s", target.Name, reason(target, err))
		}
		issuers[target.Name] = held.Issuer
	}
	return issuers, nil
}

// trustedFingerprint returns the fingerprint that names a trusted authority, or an
// empty string when the vault holds no certificate for it.
func trustedFingerprint(signers map[string]string, name string) string {
	for print, held := range signers {
		if held == name {
			return print
		}
	}
	return ""
}

// rotate signs a certificate for every server node and stops at the first node
// that does not come back. It returns the nodes that the run did not reach.
func (r *rotateRun) rotate(ctx context.Context, mesh config.Mesh, next rotateAuthority, bundle []byte, knownHosts string, signers map[string]string) (apply.Report, string, []string) {
	report := apply.Report{Mesh: mesh.Name, Nodes: []apply.NodeReport{}}
	stopped := ""
	var untouched []string
	for _, target := range mesh.Nodes {
		if target.Kind != config.KindServer {
			continue
		}
		if stopped != "" {
			untouched = append(untouched, target.Name)
			continue
		}
		one := r.rotateNode(ctx, mesh, target, next, bundle, knownHosts, signers)
		report.Add(one)
		if one.Error != "" {
			stopped = target.Name
		}
	}
	return report, stopped, untouched
}

// rotateNode replaces the certificate of one node. It reads the issuer first, so
// a node that the new authority already signed is not touched a second time,
// and the pair that the rotation replaced keeps the name of its authority.
func (r *rotateRun) rotateNode(ctx context.Context, mesh config.Mesh, target config.Node, next rotateAuthority, bundle []byte, knownHosts string, signers map[string]string) apply.NodeReport {
	report := apply.NodeReport{Name: target.Name}

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

	held, err := node.ReadCert(ctx, mesh, target, session)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	if strings.EqualFold(held.Issuer, next.fingerprint) {
		return report
	}

	// The pair that the run replaces keeps the name of the authority that
	// signed it. That authority can differ from the one on the node, because a
	// node that an earlier run did not reach still holds the old certificate.
	binary, err := r.tool()
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	previousCert, previousKey, previousFound := vault.Certificate(r.record, target.Name)
	previousAuthority := ""
	if previousFound {
		kept, err := issue.Print(ctx, binary, previousCert)
		if err != nil {
			report.Error = reason(target, err)
			return report
		}
		if strings.TrimSpace(kept.Issuer) == "" {
			report.Error = fmt.Sprintf("the certificate that the vault holds for %s names no issuer", target.Name)
			return report
		}
		previousAuthority = signerOf(signers, kept.Issuer)
	}

	// The certificate is signed from the new authority and sealed in the vault
	// before the node sees it, so phase 3 and a later apply read the same pair.
	networks, err := meshNetworks(mesh, target)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	signed, err := issue.Sign(ctx, issue.Options{
		Binary:    binary,
		Authority: issue.Authority{Cert: next.cert, Key: next.key},
		Request: issue.Request{
			Name:     target.Name,
			Networks: networks,
			Groups:   target.Groups,
			Lifetime: lifetime(mesh, target),
		},
	})
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	if previousFound {
		vault.KeepPrevious(r.record, target.Name, previousAuthority, previousCert, previousKey)
	}
	vault.SetCertificate(r.record, target.Name, signed.Cert, signed.Key)
	r.changed = true
	if err := vault.Write(r.meshDir, r.dir, r.record); err != nil {
		report.Error = reason(target, err)
		return report
	}

	result, err := apply.Run(ctx, mesh, target, apply.Files{
		Config:    rendered,
		Authority: bundle,
		Cert:      signed.Cert,
		Key:       signed.Key,
		Blocklist: r.block,
	}, session)
	report.Writes = result.Writes
	report.Action = result.Action
	report.Renewed = true
	if err != nil {
		report.Error = reason(target, err)
		return report
	}

	// The read back is the proof: the certificate on the node names the new
	// authority, and the bundle on the node holds every trusted authority.
	held, err = node.ReadCert(ctx, mesh, target, session)
	if err != nil {
		report.Error = reason(target, err)
		return report
	}
	if !strings.EqualFold(held.Issuer, next.fingerprint) {
		report.Error = fmt.Sprintf("the node reports issuer %q, %s did not sign its certificate", held.Issuer, next.name)
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

// formatRotatePhase2 writes one line for each node, and one line for each node
// that the run did not reach.
func formatRotatePhase2(report apply.Report, authority string, untouched []string) string {
	var out strings.Builder
	for _, n := range report.Nodes {
		switch {
		case n.Error != "":
			fmt.Fprintf(&out, "%s: failed: %s\n", n.Name, n.Error)
		case len(n.Writes) == 0:
			fmt.Fprintf(&out, "%s: unchanged\n", n.Name)
		default:
			fmt.Fprintf(&out, "%s: signed by %s, %s\n", n.Name, authority, applied(n))
		}
	}
	for _, name := range untouched {
		fmt.Fprintf(&out, "%s: not reached\n", name)
	}
	return out.String()
}

// authorityFingerprints maps the fingerprint of each trusted authority to its
// name. A certificate names its issuer by fingerprint, so this map turns that
// field back into the name of an authority.
func authorityFingerprints(ctx context.Context, binary string, record vault.Record, trust []string) (map[string]string, error) {
	byPrint := make(map[string]string, len(trust))
	for _, name := range trust {
		cert, _, found := vault.Certificate(record, name)
		if !found {
			continue
		}
		printed, err := issue.Print(ctx, binary, cert)
		if err != nil {
			return nil, err
		}
		byPrint[strings.ToLower(printed.Fingerprint)] = name
	}
	return byPrint, nil
}

// signerOf returns the name of the authority that a fingerprint names. A
// certificate from an authority that the mesh does not trust keeps its
// fingerprint as the name, because the pair must not lose its record.
func signerOf(signers map[string]string, fingerprint string) string {
	if name := signers[strings.ToLower(fingerprint)]; name != "" {
		return name
	}
	return fingerprint
}
