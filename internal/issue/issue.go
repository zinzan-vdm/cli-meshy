package issue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Certificate is one signed certificate, its key, and the facts that a caller
// prints or stores.
type Certificate struct {
	Cert        []byte
	Key         []byte
	NotAfter    time.Time
	Fingerprint string
}

// Authority is the key pair that signs a certificate.
type Authority struct {
	Cert []byte
	Key  []byte
}

// Request is one node that needs a certificate. Networks holds the address with
// the mesh prefix, such as 10.42.0.1/24.
type Request struct {
	Name     string
	Networks string
	Groups   []string
	Lifetime time.Duration
}

type Options struct {
	Binary    string
	Authority Authority
	Request   Request
}

// Due reports whether a node needs a new certificate. A node with no
// certificate is due. A certificate whose remaining life is at or inside the
// renewal window is due, because a run must not leave a node close to expiry.
func Due(notAfter time.Time, renewBefore time.Duration, now time.Time, force bool) bool {
	if force || notAfter.IsZero() {
		return true
	}
	return notAfter.Sub(now) <= renewBefore
}

// Sign runs the pinned nebula-cert and returns the certificate that it wrote.
func Sign(ctx context.Context, opts Options) (Certificate, error) {
	if opts.Binary == "" {
		return Certificate{}, errors.New("no nebula-cert binary")
	}
	if opts.Request.Name == "" {
		return Certificate{}, errors.New("no node name")
	}
	if opts.Request.Networks == "" {
		return Certificate{}, fmt.Errorf("no network for %s", opts.Request.Name)
	}
	if opts.Request.Lifetime <= 0 {
		return Certificate{}, errors.New("the lifetime must be more than zero")
	}

	dir, err := os.MkdirTemp("", "meshy-issue-")
	if err != nil {
		return Certificate{}, err
	}
	defer os.RemoveAll(dir)

	caCert := filepath.Join(dir, "ca.crt")
	caKey := filepath.Join(dir, "ca.key")
	certPath := filepath.Join(dir, opts.Request.Name+".crt")
	keyPath := filepath.Join(dir, opts.Request.Name+".key")
	for path, body := range map[string][]byte{caCert: opts.Authority.Cert, caKey: opts.Authority.Key} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			return Certificate{}, fmt.Errorf("write %s: %w", path, err)
		}
	}

	args := []string{
		"sign",
		"-ca-crt", caCert,
		"-ca-key", caKey,
		"-name", opts.Request.Name,
		"-networks", opts.Request.Networks,
		"-duration", hours(opts.Request.Lifetime),
		"-out-crt", certPath,
		"-out-key", keyPath,
	}
	if len(opts.Request.Groups) > 0 {
		args = append(args, "-groups", strings.Join(opts.Request.Groups, ","))
	}

	cmd := exec.CommandContext(ctx, opts.Binary, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return Certificate{}, fmt.Errorf("%s: %v: %s", filepath.Base(opts.Binary), err, strings.TrimSpace(string(out)))
	}

	cert, err := os.ReadFile(certPath)
	if err != nil {
		return Certificate{}, fmt.Errorf("read the certificate: %w", err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return Certificate{}, fmt.Errorf("read the node key: %w", err)
	}

	printed, err := printPath(ctx, opts.Binary, certPath)
	if err != nil {
		return Certificate{}, err
	}
	notAfter, err := Expiry(printed)
	if err != nil {
		return Certificate{}, err
	}

	return Certificate{
		Cert:        cert,
		Key:         key,
		NotAfter:    notAfter,
		Fingerprint: printed.Fingerprint,
	}, nil
}

// Print reads the facts of one certificate with the pinned tool.
func Print(ctx context.Context, binary string, cert []byte) (node.Cert, error) {
	dir, err := os.MkdirTemp("", "meshy-print-")
	if err != nil {
		return node.Cert{}, err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "cert.crt")
	if err := os.WriteFile(path, cert, 0o600); err != nil {
		return node.Cert{}, fmt.Errorf("write %s: %w", path, err)
	}
	return printPath(ctx, binary, path)
}

// Expiry reads the expiry that the tool printed for a certificate.
func Expiry(cert node.Cert) (time.Time, error) {
	notAfter, err := time.Parse(time.RFC3339, cert.NotAfter)
	if err != nil {
		return time.Time{}, fmt.Errorf("the certificate expiry %q is not readable", cert.NotAfter)
	}
	return notAfter, nil
}

func printPath(ctx context.Context, binary, path string) (node.Cert, error) {
	out, err := exec.CommandContext(ctx, binary, "print", "-json", "-path", path).Output()
	if err != nil {
		return node.Cert{}, fmt.Errorf("%s print: %w", filepath.Base(binary), err)
	}
	return node.ParseCert(string(out))
}

// hours writes a lifetime in the unit that nebula-cert reads. A year is 365
// days in this project, so one year is 8760h.
func hours(d time.Duration) string {
	h := int64(d / time.Hour)
	if h < 1 {
		h = 1
	}
	return fmt.Sprintf("%dh", h)
}
