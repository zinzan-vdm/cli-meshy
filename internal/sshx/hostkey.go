package sshx

import (
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// HostKeys builds the host key check from a known hosts file. There is no
// path that skips the check.
func HostKeys(path string) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, fmt.Errorf("no known hosts file, so no host key can be checked")
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("known hosts file %s: %w", path, err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := cb(hostname, remote, key); err != nil {
			return fmt.Errorf("host key for %s does not match %s: %w", hostname, path, err)
		}
		return nil
	}, nil
}
