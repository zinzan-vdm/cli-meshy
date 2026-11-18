package vault

import (
	"sort"
	"strings"
)

// A node and an authority both hold a certificate and the key that matches it.
// The record names them after the holder, so ca-1.key and core-1.key are two
// different keys in one vault.
const (
	CertSuffix = ".crt"
	KeySuffix  = ".key"
	// RevokedSuffix marks a certificate that the mesh has blocked. The file
	// holds the fingerprint, which is what the blocklist of a node carries.
	RevokedSuffix = ".revoked"
)

// Certificate returns the certificate and the key that the record holds under
// name. A record that holds one of the two is not a certificate.
func Certificate(record Record, name string) (cert, key []byte, ok bool) {
	cert, certOK := record[name+CertSuffix]
	key, keyOK := record[name+KeySuffix]
	if !certOK || !keyOK {
		return nil, nil, false
	}
	return cert, key, true
}

// SetCertificate stores a certificate and its key in the record under name.
func SetCertificate(record Record, name string, cert, key []byte) {
	record[name+CertSuffix] = cert
	record[name+KeySuffix] = key
}

// DeleteCertificate removes the certificate and the key that the record holds
// under name. A name that holds one of the two loses it.
func DeleteCertificate(record Record, name string) {
	delete(record, name+CertSuffix)
	delete(record, name+KeySuffix)
}

// A rotation keeps the certificate that a node held before the new one, under
// the name of the authority that signed it. The suffix does not end in .crt, so
// a kept certificate is not a certificate that the record offers for a node and
// the revocation never reads it as one.
const previousMarker = ".previous."

// KeepPrevious stores the pair that a rotation replaces, so the old certificate
// stays in the vault and stays usable until phase 3 drops its authority.
func KeepPrevious(record Record, name, authority string, cert, key []byte) {
	record[name+CertSuffix+previousMarker+authority] = cert
	record[name+KeySuffix+previousMarker+authority] = key
}

// Previous returns the pair that a node held before the authority signed the
// current one.
func Previous(record Record, name, authority string) (cert, key []byte, ok bool) {
	cert, certOK := record[name+CertSuffix+previousMarker+authority]
	key, keyOK := record[name+KeySuffix+previousMarker+authority]
	if !certOK || !keyOK {
		return nil, nil, false
	}
	return cert, key, true
}

// PreviousAuthorities returns the authorities whose kept pairs the record holds
// for one node, in sort order. Phase 3 needs the list, because it drops the
// certificates of the authority that it removes.
func PreviousAuthorities(record Record, name string) []string {
	prefix := name + CertSuffix + previousMarker
	var names []string
	for key := range record {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		names = append(names, strings.TrimPrefix(key, prefix))
	}
	sort.Strings(names)
	return names
}

// Certificates returns every name that the record holds a certificate for, in
// sort order. The names are a mix of nodes and authorities, so a caller that
// wants nodes must set aside the names that the mesh file trusts.
func Certificates(record Record) []string {
	names := make([]string, 0, len(record))
	for name := range record {
		if !strings.HasSuffix(name, CertSuffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(name, CertSuffix))
	}
	sort.Strings(names)
	return names
}

// Blocked is one certificate that the mesh has blocked. The fingerprint stays in
// the record after the certificate itself leaves, because a node that returns
// must not be trusted again.
type Blocked struct {
	Name        string
	Fingerprint string
}

// Revoked returns every certificate that the record marks as blocked, in sort
// order of name. An entry that holds no fingerprint is not a block.
func Revoked(record Record) []Blocked {
	names := make([]string, 0, len(record))
	for name := range record {
		if !strings.HasSuffix(name, RevokedSuffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(name, RevokedSuffix))
	}
	sort.Strings(names)

	blocked := make([]Blocked, 0, len(names))
	for _, name := range names {
		fingerprint := strings.TrimSpace(string(record[name+RevokedSuffix]))
		if fingerprint == "" {
			continue
		}
		blocked = append(blocked, Blocked{Name: name, Fingerprint: fingerprint})
	}
	return blocked
}

// SetRevoked marks a certificate as blocked and keeps its fingerprint.
func SetRevoked(record Record, name, fingerprint string) {
	record[name+RevokedSuffix] = []byte(fingerprint + "\n")
}
