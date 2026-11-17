package vault

// A node and an authority both hold a certificate and the key that matches it.
// The record names them after the holder, so ca-1.key and core-1.key are two
// different keys in one vault.
const (
	CertSuffix = ".crt"
	KeySuffix  = ".key"
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
