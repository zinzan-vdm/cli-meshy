package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

func newKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	return k
}

func writeKnownHosts(t *testing.T, key ssh.PublicKey) string {
	t.Helper()
	line := "[203.0.113.10]:22 " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + "\n"
	p := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
		t.Fatalf("write known hosts: %v", err)
	}
	return p
}

func TestHostKeysAcceptsTheListedKey(t *testing.T) {
	key := newKey(t)
	path := writeKnownHosts(t, key)

	cb, err := HostKeys(path)
	if err != nil {
		t.Fatalf("HostKeys: %v", err)
	}
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 22}
	if err := cb("[203.0.113.10]:22", addr, key); err != nil {
		t.Errorf("known key rejected: %v", err)
	}
}

func TestHostKeysRejectsAnotherKey(t *testing.T) {
	path := writeKnownHosts(t, newKey(t))
	other := newKey(t)

	cb, err := HostKeys(path)
	if err != nil {
		t.Fatalf("HostKeys: %v", err)
	}
	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.10"), Port: 22}
	err = cb("[203.0.113.10]:22", addr, other)
	if err == nil {
		t.Fatal("a different host key was accepted")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error does not say the key does not match: %v", err)
	}
}

func TestHostKeysNeedsAFile(t *testing.T) {
	if _, err := HostKeys(""); err == nil {
		t.Fatal("an empty known hosts path was accepted")
	}
	if _, err := HostKeys(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a missing known hosts file was accepted")
	}
}

func TestFromNodeFillsTheDefaults(t *testing.T) {
	n := config.Node{Name: "core-1", Kind: config.KindServer, SSH: &config.SSH{User: "meshy", Host: "203.0.113.10"}}
	cfg, err := FromNode(n, "/etc/meshy/known_hosts")
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	if cfg.Port != 22 {
		t.Errorf("port = %d, want 22", cfg.Port)
	}
	if cfg.User != "meshy" || cfg.Host != "203.0.113.10" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

// writeKeyFile writes a private key and returns its path.
func writeKeyFile(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	p := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(p, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return p
}

func TestAuthMethodsReadsTheKeyFile(t *testing.T) {
	methods, closers, err := authMethods(writeKeyFile(t))
	if err != nil {
		t.Fatalf("authMethods: %v", err)
	}
	if len(methods) != 1 {
		t.Errorf("%d methods, want 1", len(methods))
	}
	if len(closers) != 0 {
		t.Errorf("%d closers, want 0", len(closers))
	}
}

func TestAuthMethodsWithoutAKeyOrAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	if _, _, err := authMethods(""); err == nil {
		t.Fatal("authMethods found a way to authenticate with no key and no agent")
	}
}
