package node

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// State is what a node actually runs. It is read, never written.
type State struct {
	Name        string
	Version     string
	Service     string
	Interface   bool
	ConfigHash  string
	Cert        Cert
	Authorities []Cert
}

// Cert is the part of a certificate that the plan compares.
type Cert struct {
	Present     bool
	Name        string
	Fingerprint string
	// Issuer is the fingerprint of the authority that signed the certificate.
	// A rotation reads it back, because it is the proof of which authority
	// signed the certificate that the node holds.
	Issuer    string
	Groups    []string
	Addresses []string
	NotAfter  string
	IsCA      bool
}

var versionPattern = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

// ParseVersion reads the release from `nebula -version`.
func ParseVersion(out string) (string, error) {
	m := versionPattern.FindStringSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("no version in %q", strings.TrimSpace(out))
	}
	return m[1], nil
}

// ParseService reads the unit state from `systemctl is-active`.
func ParseService(out string) string {
	s := strings.TrimSpace(out)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// ParseHash reads the digest from `sha256sum`.
func ParseHash(out string) (string, error) {
	f := strings.Fields(strings.TrimSpace(out))
	if len(f) == 0 || len(f[0]) != 64 {
		return "", fmt.Errorf("no sha256 digest in %q", strings.TrimSpace(out))
	}
	return strings.ToLower(f[0]), nil
}

// ParseCert reads the certificate details from `nebula-cert print -json`. A
// certificate file holds one document, and a trust bundle holds several, so this
// function returns the first document and ParseCerts returns all of them.
func ParseCert(out string) (Cert, error) {
	certs, err := ParseCerts(out)
	if err != nil {
		return Cert{}, err
	}
	return certs[0], nil
}

// ParseCerts reads every certificate that `nebula-cert print -json` prints. The
// tool prints an array, and the array holds one entry for each PEM block in the
// file, so ca.crt with two authorities prints two.
func ParseCerts(out string) ([]Cert, error) {
	var list []certDoc
	if err := json.Unmarshal([]byte(out), &list); err == nil {
		if len(list) == 0 {
			return nil, fmt.Errorf("certificate has no fingerprint")
		}
		return docs(list)
	}

	var doc certDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, fmt.Errorf("certificate is not readable: %w", err)
	}
	return docs([]certDoc{doc})
}

func docs(list []certDoc) ([]Cert, error) {
	certs := make([]Cert, 0, len(list))
	for _, doc := range list {
		cert, err := doc.cert()
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// certDoc is one entry of the array that nebula-cert prints. The tool prints an
// array of one, and older versions printed the object alone.
type certDoc struct {
	Details struct {
		Name     string   `json:"name"`
		Issuer   string   `json:"issuer"`
		Groups   []string `json:"groups"`
		Ips      []string `json:"ips"`
		NotAfter string   `json:"notAfter"`
		IsCA     bool     `json:"isCA"`
	} `json:"details"`
	Fingerprint string `json:"fingerprint"`
}

func (d certDoc) cert() (Cert, error) {
	if d.Fingerprint == "" {
		return Cert{}, fmt.Errorf("certificate has no fingerprint")
	}
	return Cert{
		Present:     true,
		Name:        d.Details.Name,
		Fingerprint: d.Fingerprint,
		Issuer:      d.Details.Issuer,
		Groups:      d.Details.Groups,
		Addresses:   d.Details.Ips,
		NotAfter:    d.Details.NotAfter,
		IsCA:        d.Details.IsCA,
	}, nil
}
