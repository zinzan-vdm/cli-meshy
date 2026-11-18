package sshx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

type reply struct {
	stdout string
	stderr string
	code   int
}

type handler func(cmd string, stdin []byte) reply

// startWriteServer returns a client that talks to an SSH server in this
// process, on the loopback address that the other tests in this package use.
func startWriteServer(t *testing.T, handle handler) *Client {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveWriteConn(conn, cfg, handle)
		}
	}()

	addr := ln.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            "meshy",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	client := &Client{client: ssh.NewClient(c, chans, reqs), host: addr}
	t.Cleanup(func() { client.Close() })
	return client
}

func serveWriteConn(conn net.Conn, cfg *ssh.ServerConfig, handle handler) {
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

				stdin, _ := io.ReadAll(ch)
				out := handle(payload.Command, stdin)
				ch.Write([]byte(out.stdout))
				ch.Stderr().Write([]byte(out.stderr))
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{uint32(out.code)}))
				return
			}
		}()
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestWriteSendsTheBytesOnStdin(t *testing.T) {
	const path = "/etc/nebula/meshy-homelab/config.yml.tmp"
	body := []byte("pki:\n  ca: /etc/nebula/meshy-homelab/ca.crt\n")

	var gotCommand string
	var gotStdin []byte
	client := startWriteServer(t, func(cmd string, stdin []byte) reply {
		gotCommand = cmd
		gotStdin = stdin
		return reply{}
	})

	if err := client.Write(testContext(t), path, body, 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if string(gotStdin) != string(body) {
		t.Errorf("stdin = %q, want %q", gotStdin, body)
	}
	if !strings.Contains(gotCommand, "cat > '"+path+"'") {
		t.Errorf("the command does not write the path from stdin: %q", gotCommand)
	}
	if !strings.Contains(gotCommand, "chmod 0600 '"+path+"'") {
		t.Errorf("the command does not set the mode: %q", gotCommand)
	}
	for _, banned := range []string{"scp", "sftp", "<<"} {
		if strings.Contains(gotCommand, banned) {
			t.Errorf("the command uses %q: %q", banned, gotCommand)
		}
	}
}

func TestWriteKeepsTheModeItWasGiven(t *testing.T) {
	var gotCommand string
	client := startWriteServer(t, func(cmd string, _ []byte) reply {
		gotCommand = cmd
		return reply{}
	})

	if err := client.Write(testContext(t), "/etc/nebula/meshy-homelab/ca.crt.tmp", []byte("cert"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(gotCommand, "chmod 0644") {
		t.Errorf("the command does not carry the mode 0644: %q", gotCommand)
	}
}

func TestWriteNamesThePathWhenItFails(t *testing.T) {
	client := startWriteServer(t, func(string, []byte) reply {
		return reply{stderr: "No space left on device", code: 1}
	})

	err := client.Write(testContext(t), "/etc/nebula/meshy-homelab/config.yml.tmp", []byte("x"), 0o600)
	if err == nil {
		t.Fatal("Write succeeded although the node refused it")
	}
	if !strings.Contains(err.Error(), "config.yml.tmp") || !strings.Contains(err.Error(), "No space left") {
		t.Errorf("the error does not name the path and the reason: %v", err)
	}
}

func TestReadReturnsTheBytes(t *testing.T) {
	var gotCommand string
	client := startWriteServer(t, func(cmd string, _ []byte) reply {
		gotCommand = cmd
		return reply{stdout: "pki:\n"}
	})

	body, err := client.Read(testContext(t), "/etc/nebula/meshy-homelab/config.yml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(body) != "pki:\n" {
		t.Errorf("body = %q", body)
	}
	if !strings.Contains(gotCommand, "cat '/etc/nebula/meshy-homelab/config.yml'") {
		t.Errorf("the command does not read the path: %q", gotCommand)
	}
}

func TestReadReportsAMissingFile(t *testing.T) {
	client := startWriteServer(t, func(string, []byte) reply {
		return reply{code: missingExit}
	})

	_, err := client.Read(testContext(t), "/etc/nebula/meshy-homelab/config.yml")
	if !errors.Is(err, node.ErrNoFile) {
		t.Fatalf("error = %v, want node.ErrNoFile", err)
	}
}

func TestReadReportsAFailedRead(t *testing.T) {
	client := startWriteServer(t, func(string, []byte) reply {
		return reply{stderr: "Permission denied", code: 1}
	})

	_, err := client.Read(testContext(t), "/etc/nebula/meshy-homelab/config.yml")
	if err == nil {
		t.Fatal("Read succeeded although the node refused it")
	}
	if errors.Is(err, node.ErrNoFile) {
		t.Error("a failed read is reported as a missing file")
	}
	if !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("the error does not name the reason: %v", err)
	}
}
