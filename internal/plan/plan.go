package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Kind is the field of a node that holds a difference.
type Kind string

const (
	KindCertificate Kind = "certificate"
	KindConfig      Kind = "config"
	KindBinary      Kind = "binary"
	KindService     Kind = "service"
	KindInterface   Kind = "interface"
	KindRevoked     Kind = "revoked"
)

// Revoked is one certificate that the vault holds and mesh.yml no longer names.
// The plan reports it, because the run blocks it on every node.
type Revoked struct {
	Name        string
	Fingerprint string
}

// Change is one difference between the mesh file and a node.
type Change struct {
	Kind   Kind   `json:"kind"`
	Detail string `json:"detail"`
}

// Node is one node in the plan. A node with no change and no error matches the
// mesh file. Unread marks a user node, which is never read over SSH.
type Node struct {
	Name    string   `json:"name"`
	Unread  bool     `json:"unread,omitempty"`
	Error   string   `json:"error,omitempty"`
	Changes []Change `json:"changes"`
}

// Report is the plan for one mesh. A workflow parses it, so the field names are
// the schema.
type Report struct {
	Mesh    string `json:"mesh"`
	Nodes   []Node `json:"nodes"`
	Changes int    `json:"changes"`
}

// Clean reports whether the mesh file and the nodes agree. A node that could
// not be read is not clean, because the plan cannot say that it matches.
// Failed reports that at least one node could not be read, so the plan is
// incomplete.
func (r Report) Failed() bool {
	for _, n := range r.Nodes {
		if n.Error != "" {
			return true
		}
	}
	return false
}

// Clean reports that every node matches the mesh file.
func (r Report) Clean() bool {
	if r.Changes > 0 {
		return false
	}
	for _, n := range r.Nodes {
		if n.Error != "" {
			return false
		}
	}
	return true
}

// Reading is what one node returned, or the error that stopped the read.
type Reading struct {
	Name  string
	State node.State
	Err   error
}

// Input holds the mesh file, what the nodes returned, and the certificate
// records from the vault. Certs may be nil, which is the case while no vault is
// readable. Now is an argument because the plan reads no clock.
type Input struct {
	Mesh     config.Mesh
	Readings []Reading
	Certs    map[string]node.Cert
	Revoked  []Revoked
	Now      time.Time
}

// Compute compares the mesh file with the readings and returns every
// difference, sorted by node name and then by kind.
func Compute(in Input) Report {
	readings := make(map[string]Reading, len(in.Readings))
	for _, r := range in.Readings {
		readings[r.Name] = r
	}

	nodes := make([]config.Node, len(in.Mesh.Nodes))
	copy(nodes, in.Mesh.Nodes)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	named := make(map[string]bool, len(nodes))
	rows := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		named[n.Name] = true
		r, found := readings[n.Name]
		rows = append(rows, compare(in, n, r, found))
	}

	// A read that names no node in the file still holds a fact that the plan
	// must not drop, because a silent failure reads as agreement.
	for _, r := range in.Readings {
		if named[r.Name] {
			continue
		}
		row := Node{Name: r.Name, Changes: []Change{}}
		if r.Err != nil {
			row.Error = "read failed: " + r.Err.Error()
		} else {
			row.Error = "read a node that " + config.FileName + " does not name"
		}
		rows = append(rows, row)
	}

	// A certificate that the vault holds and the file does not name is a
	// revocation, and the run acts on it, so the plan names it.
	for _, revoked := range in.Revoked {
		if named[revoked.Name] {
			continue
		}
		detail := "blocklist, fingerprint " + revoked.Fingerprint
		rows = append(rows, Node{
			Name:    revoked.Name,
			Changes: []Change{{Kind: KindRevoked, Detail: detail}},
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	report := Report{Mesh: in.Mesh.Name, Nodes: rows}
	for _, row := range rows {
		report.Changes += len(row.Changes)
	}
	return report
}

func compare(in Input, n config.Node, r Reading, found bool) Node {
	row := Node{Name: n.Name, Changes: []Change{}}

	if n.Kind == config.KindUser {
		row.Unread = true
		if record, ok := in.Certs[n.Name]; ok {
			row.Changes = certFacts(in.Mesh, n, record, in.Now)
		}
	} else {
		switch {
		case !found:
			row.Error = "read failed: the node was not read"
		case r.Err != nil:
			row.Error = "read failed: " + strings.TrimPrefix(r.Err.Error(), "node "+n.Name+": ")
		default:
			row.Changes = serverFacts(in, n, r.State)
		}
	}

	sort.Slice(row.Changes, func(i, j int) bool {
		if row.Changes[i].Kind != row.Changes[j].Kind {
			return row.Changes[i].Kind < row.Changes[j].Kind
		}
		return row.Changes[i].Detail < row.Changes[j].Detail
	})
	return row
}

func serverFacts(in Input, n config.Node, st node.State) []Change {
	changes := certFacts(in.Mesh, n, st.Cert, in.Now)

	if record, found := in.Certs[n.Name]; found && st.Cert.Present &&
		record.Fingerprint != "" && record.Fingerprint != st.Cert.Fingerprint {
		changes = append(changes, Change{Kind: KindCertificate, Detail: "differs from the vault"})
	}

	changes = append(changes, configFacts(in.Mesh, n, st)...)
	changes = append(changes, binaryFacts(in.Mesh, st)...)
	changes = append(changes, serviceFacts(in.Mesh, st)...)
	changes = append(changes, interfaceFacts(in.Mesh, st)...)
	changes = append(changes, authorityFacts(in.Mesh, st)...)
	return changes
}

// certFacts compares the name, the groups, the addresses, and the remaining
// life of a certificate with the mesh file. Nebula takes the overlay address
// from the certificate, so a wrong address is a certificate fact.
func certFacts(m config.Mesh, n config.Node, cert node.Cert, now time.Time) []Change {
	if !cert.Present {
		return []Change{{Kind: KindCertificate, Detail: "no certificate"}}
	}

	var changes []Change
	if cert.Name != n.Name {
		changes = append(changes, Change{
			Kind:   KindCertificate,
			Detail: fmt.Sprintf("certificate names %s, mesh names %s", cert.Name, n.Name),
		})
	}
	if got, want := list(cert.Groups), list(n.Groups); got != want {
		changes = append(changes, Change{
			Kind:   KindCertificate,
			Detail: fmt.Sprintf("groups %s, mesh %s", got, want),
		})
	}
	if got, want := list(addresses(cert.Addresses)), list([]string{n.IP}); got != want {
		changes = append(changes, Change{
			Kind:   KindCertificate,
			Detail: fmt.Sprintf("address %s, mesh %s", got, want),
		})
	}
	if change, found := expiryFacts(cert, m.Certificates.RenewBefore.Duration, now); found {
		changes = append(changes, change)
	}
	return changes
}

func expiryFacts(cert node.Cert, renewBefore time.Duration, now time.Time) (Change, bool) {
	if cert.NotAfter == "" {
		return Change{Kind: KindCertificate, Detail: "no expiry"}, true
	}
	notAfter, err := time.Parse(time.RFC3339, cert.NotAfter)
	if err != nil {
		return Change{Kind: KindCertificate, Detail: fmt.Sprintf("expiry %q is not readable", cert.NotAfter)}, true
	}

	remaining := notAfter.Sub(now)
	switch {
	case remaining <= 0:
		return Change{Kind: KindCertificate, Detail: "expired " + span(-remaining) + " ago"}, true
	case remaining < renewBefore:
		return Change{Kind: KindCertificate, Detail: "expires in " + span(remaining) + ", renews"}, true
	}
	return Change{}, false
}

// span writes a remaining life in the largest unit that keeps it readable.
func span(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
}

// list sorts a copy and joins it, so two sets compare by content and not by the
// order the file or the certificate happened to write.
func list(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	sorted := make([]string, len(items))
	copy(sorted, items)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

// addresses drops the mesh prefix from a certificate address, because the
// certificate may write 10.42.0.2/24 where the file writes 10.42.0.2.
func addresses(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if i := strings.IndexByte(item, '/'); i >= 0 {
			item = item[:i]
		}
		out = append(out, item)
	}
	return out
}

// configFacts renders the config the node must run and compares its hash with
// the hash the read returned.
func configFacts(m config.Mesh, n config.Node, st node.State) []Change {
	body, err := nebula.Render(m, n)
	if err != nil {
		return []Change{{Kind: KindConfig, Detail: "cannot render: " + err.Error()}}
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != st.ConfigHash {
		return []Change{{Kind: KindConfig, Detail: "differs"}}
	}
	return nil
}

func binaryFacts(m config.Mesh, st node.State) []Change {
	if st.Version == strings.TrimPrefix(m.Nebula.Version, "v") {
		return nil
	}
	return []Change{{
		Kind:   KindBinary,
		Detail: fmt.Sprintf("runs v%s, pinned %s", st.Version, m.Nebula.Version),
	}}
}

func serviceFacts(m config.Mesh, st node.State) []Change {
	if st.Service == "active" {
		return nil
	}
	state := st.Service
	if state == "" {
		state = "unknown"
	}
	return []Change{{
		Kind:   KindService,
		Detail: fmt.Sprintf("unit %s is %s", node.Unit(m.Name), state),
	}}
}

func interfaceFacts(m config.Mesh, st node.State) []Change {
	if st.Interface {
		return nil
	}
	return []Change{{
		Kind:   KindInterface,
		Detail: fmt.Sprintf("device %s is absent", m.Interface),
	}}
}

// authorityFacts compares the authorities in the trust bundle on the node with
// the authorities that mesh.yml trusts. The bundle is the whole set, so a name
// that is missing on the node and a name that the mesh does not trust are two
// different differences.
func authorityFacts(m config.Mesh, st node.State) []Change {
	if len(st.Authorities) == 0 {
		return nil
	}

	onNode := make(map[string]bool, len(st.Authorities))
	for _, cert := range st.Authorities {
		onNode[cert.Name] = true
	}
	trusted := make(map[string]bool, len(m.CA.Trust))
	for _, name := range m.CA.Trust {
		trusted[name] = true
	}

	var changes []Change
	for _, name := range m.CA.Trust {
		if !onNode[name] {
			changes = append(changes, Change{
				Kind:   KindCertificate,
				Detail: fmt.Sprintf("the node does not trust %s", name),
			})
		}
	}
	for _, cert := range st.Authorities {
		if !trusted[cert.Name] {
			changes = append(changes, Change{
				Kind:   KindCertificate,
				Detail: fmt.Sprintf("authority %s is not trusted", cert.Name),
			})
		}
	}
	return changes
}
