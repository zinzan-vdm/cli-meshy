package sshx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx/sshtest"
)

// server is a local SSH server that answers the read commands.
type server struct {
	addr       string
	knownHosts string
	stop       func()
}

func startServer(t *testing.T, answers map[string]string, failures map[string]int) server {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("client key: %v", err)
	}
	clientKey, err := ssh.NewPublicKey(clientPub)
	if err != nil {
		t.Fatalf("client public key: %v", err)
	}

	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(clientKey.Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown key")
		},
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn, cfg, answers, failures)
		}
	}()

	// the client key on disk
	block, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("MESHY_SSH_KEY", keyPath)

	// known hosts for the port the server got
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	line := "[127.0.0.1]:" + port + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostSigner.PublicKey()))) + "\n"
	kh := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(kh, []byte(line), 0o600); err != nil {
		t.Fatalf("write known hosts: %v", err)
	}

	return server{addr: ln.Addr().String(), knownHosts: kh, stop: func() { ln.Close() }}
}

func serve(conn net.Conn, cfg *ssh.ServerConfig, answers map[string]string, failures map[string]int) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer ch.Close()
			for req := range requests {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				ssh.Unmarshal(req.Payload, &payload)
				req.Reply(true, nil)
				if out, ok := answers[payload.Command]; ok {
					ch.Write([]byte(out))
				}
				code := uint32(0)
				if c, ok := failures[payload.Command]; ok {
					code = uint32(c)
				}
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{code}))
				return
			}
		}()
	}
}

func TestReadOverALocalServer(t *testing.T) {
	m := config.Mesh{
		Name:      "homelab",
		Interface: "meshy0",
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1", SSH: &config.SSH{User: "meshy", Host: "127.0.0.1"}},
		},
	}
	n := m.Nodes[0]

	answers := map[string]string{
		"nebula -version":                          "Version: 1.12.0\n",
		"systemctl is-active nebula@meshy-homelab": "active\n",
		"nebula-cert print -json -path /etc/nebula/meshy-homelab/core-1.crt": `{"details":{"name":"core-1","groups":["core"],"ips":["10.42.0.1/24"],"notAfter":"2027-11-17T20:00:00Z"},"fingerprint":"aa11bb22"}`,
		"ip link show meshy0":                                            "2: meshy0: <POINTOPOINT>\n",
		"sha256sum /etc/nebula/meshy-homelab/config.yml":                 strings.Repeat("cd", 32) + "  config.yml\n",
		"nebula-cert print -json -path /etc/nebula/meshy-homelab/ca.crt": `{"details":{"name":"ca-1","isCA":true},"fingerprint":"ffee0011"}`,
	}

	srv := startServer(t, answers, nil)
	defer srv.stop()

	_, port, _ := net.SplitHostPort(srv.addr)
	p, _ := strconv.Atoi(port)
	n.SSH.Port = p

	cfg, err := FromNode(n, srv.knownHosts)
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	c, err := Dial(cfg)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	st, err := node.Read(context.Background(), m, n, c)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if st.Version != "1.12.0" || st.Service != "active" || !st.Interface {
		t.Errorf("unexpected state: %+v", st)
	}
	if st.ConfigHash != strings.Repeat("cd", 32) {
		t.Errorf("config hash = %q", st.ConfigHash)
	}
	if st.Cert.Fingerprint != "aa11bb22" || st.Authority.Fingerprint != "ffee0011" {
		t.Errorf("certificates = %+v %+v", st.Cert, st.Authority)
	}
}

func TestRunKeepsANonZeroExit(t *testing.T) {
	srv := startServer(t, map[string]string{"false": ""}, map[string]int{"false": 3})
	defer srv.stop()

	_, port, _ := net.SplitHostPort(srv.addr)
	p, _ := strconv.Atoi(port)
	n := config.Node{Name: "core-1", Kind: config.KindServer, SSH: &config.SSH{User: "meshy", Host: "127.0.0.1", Port: p}}

	cfg, err := FromNode(n, srv.knownHosts)
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	c, err := Dial(cfg)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	res, err := c.Run(context.Background(), "false")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", res.ExitCode)
	}
}

func TestDialFailsOnAnUnknownHostKey(t *testing.T) {
	srv := startServer(t, map[string]string{}, nil)
	defer srv.stop()

	// a known hosts file that holds another key
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	otherSigner, _ := ssh.NewSignerFromKey(other)
	_, port, _ := net.SplitHostPort(srv.addr)
	line := "[127.0.0.1]:" + port + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(otherSigner.PublicKey()))) + "\n"
	kh := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(kh, []byte(line), 0o600); err != nil {
		t.Fatalf("write known hosts: %v", err)
	}

	p, _ := strconv.Atoi(port)
	n := config.Node{Name: "core-1", Kind: config.KindServer, SSH: &config.SSH{User: "meshy", Host: "127.0.0.1", Port: p}}
	cfg, err := FromNode(n, kh)
	if err != nil {
		t.Fatalf("FromNode: %v", err)
	}
	if _, err := Dial(cfg); err == nil {
		t.Fatal("Dial accepted a host key that does not match")
	}
}

func TestReadNamesAMissingCertificate(t *testing.T) {
	m := config.Mesh{
		Name:      "homelab",
		Interface: "meshy0",
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1"},
		},
	}
	srv := sshtest.New(t, sshtest.WithVersion("1.12.0"))

	_, err := node.Read(context.Background(), m, m.Nodes[0], srv.Connect())
	if err == nil {
		t.Fatal("Read succeeded on a node with no certificate")
	}
	want := "no certificate at " + node.Dir(m.Name) + "/core-1.crt"
	if !strings.Contains(err.Error(), want) {
		t.Errorf("the error does not say the certificate is absent: %v", err)
	}
}
