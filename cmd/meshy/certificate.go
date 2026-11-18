package main

import (
	"context"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/issue"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// certifier returns the certificate and the key that one node must hold. apply
// and bundle share it, so the renewal window has one definition and one code
// path.
type certifier struct {
	mesh    *config.Mesh
	meshDir string
	record  vault.Record
	binary  string
	now     time.Time
	changed bool
}

// tool returns the pinned nebula-cert. A run that issues or revokes needs it.
func (c *certifier) tool() (string, error) {
	if c.binary != "" {
		return c.binary, nil
	}
	binary, err := ca.Find(c.meshDir, c.mesh.Nebula.Version)
	if err != nil {
		return "", err
	}
	c.binary = binary
	return binary, nil
}

// authority returns the key pair that signs. The vault must hold it, because a
// run that renews a certificate needs the authority key.
func (c *certifier) authority() issue.Authority {
	cert, key, _ := vault.Certificate(c.record, c.mesh.CA.Active)
	return issue.Authority{Cert: cert, Key: key}
}

// certificate returns the certificate and the key that the node must hold. It
// signs a new certificate when the vault holds none or when the one it holds is
// inside the renewal window, and it keeps the address and the groups, so the
// node needs a reload and not a restart.
func (c *certifier) certificate(ctx context.Context, target config.Node) (cert, key []byte, renewed bool, err error) {
	binary, err := c.tool()
	if err != nil {
		return nil, nil, false, err
	}

	if current, currentKey, ok := vault.Certificate(c.record, target.Name); ok && !due(ctx, binary, current, *c.mesh, c.now) {
		return current, currentKey, false, nil
	}

	networks, err := meshNetworks(*c.mesh, target)
	if err != nil {
		return nil, nil, false, err
	}
	signed, err := issue.Sign(ctx, issue.Options{
		Binary:    binary,
		Authority: c.authority(),
		Request: issue.Request{
			Name:     target.Name,
			Networks: networks,
			Groups:   target.Groups,
			Lifetime: lifetime(*c.mesh, target),
		},
	})
	if err != nil {
		return nil, nil, false, err
	}

	vault.SetCertificate(c.record, target.Name, signed.Cert, signed.Key)
	c.changed = true
	return signed.Cert, signed.Key, true, nil
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
