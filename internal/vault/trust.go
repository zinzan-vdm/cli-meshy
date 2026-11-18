package vault

import (
	"bytes"
	"errors"
	"fmt"
)

// TrustBundle returns the bytes that a node holds in ca.crt: the certificate of
// every authority in names, in that order, with one file and one newline between
// them. Nebula reads every PEM block in the file that pki.ca names, so one file
// carries the whole trust bundle. See docs/adr/0011.
func TrustBundle(record Record, names []string) ([]byte, error) {
	if len(names) == 0 {
		return nil, errors.New("the mesh names no trusted authority")
	}

	var out bytes.Buffer
	for _, name := range names {
		cert, ok := record[name+CertSuffix]
		if !ok || len(bytes.TrimSpace(cert)) == 0 {
			return nil, fmt.Errorf("the vault holds no certificate for authority %s", name)
		}
		out.Write(bytes.TrimRight(cert, "\n"))
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}
