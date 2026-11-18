// Package revoke finds the certificates that a mesh no longer names and turns
// them into the blocklist that every node must hold.
package revoke

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// Node is one certificate that mesh.yml no longer names.
type Node struct {
	Name        string
	Fingerprint string
}

// Names returns the names that the record holds a certificate for and mesh.yml
// does not name. An authority is not a node, so its name is not a revocation.
func Names(record vault.Record, m config.Mesh) []string {
	named := make(map[string]bool, len(m.Nodes))
	for _, n := range m.Nodes {
		named[n.Name] = true
	}
	trusted := make(map[string]bool, len(m.CA.Trust))
	for _, name := range m.CA.Trust {
		trusted[name] = true
	}
	trusted[m.CA.Active] = true

	var names []string
	for _, name := range vault.Certificates(record) {
		if named[name] || trusted[name] {
			continue
		}
		names = append(names, name)
	}
	return names
}

// List returns the revoked nodes with the fingerprint of each certificate. The
// fingerprints come from the pinned tool, because a blocklist holds the
// fingerprint that the tool prints and not a digest of the file. A certificate
// that signs other certificates is an authority, and the run never blocks one.
func List(ctx context.Context, binary string, record vault.Record, m config.Mesh) ([]Node, error) {
	names := Names(record, m)
	nodes := make([]Node, 0, len(names))
	for _, name := range names {
		cert, _, ok := vault.Certificate(record, name)
		if !ok {
			continue
		}
		printed, err := issue.Print(ctx, binary, cert)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", name, err)
		}
		if printed.IsCA {
			continue
		}
		if printed.Fingerprint == "" {
			return nil, fmt.Errorf("node %s: the certificate has no fingerprint", name)
		}
		nodes = append(nodes, Node{Name: name, Fingerprint: printed.Fingerprint})
	}
	return nodes, nil
}

// Text renders the blocklist that a node reads: one fingerprint per line, in
// sort order, and no line when nothing is blocked. The record holds the
// fingerprints that the mesh already blocked, so a block outlives the run that
// made it.
func Text(record vault.Record, pending []Node) []byte {
	seen := make(map[string]bool, len(pending))
	prints := make([]string, 0, len(pending))
	for _, n := range pending {
		if n.Fingerprint == "" || seen[n.Fingerprint] {
			continue
		}
		seen[n.Fingerprint] = true
		prints = append(prints, n.Fingerprint)
	}
	for _, blocked := range vault.Revoked(record) {
		if seen[blocked.Fingerprint] {
			continue
		}
		seen[blocked.Fingerprint] = true
		prints = append(prints, blocked.Fingerprint)
	}
	if len(prints) == 0 {
		return nil
	}
	sort.Strings(prints)

	var out strings.Builder
	for _, print := range prints {
		out.WriteString(print)
		out.WriteString("\n")
	}
	return []byte(out.String())
}
