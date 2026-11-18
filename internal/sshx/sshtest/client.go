package sshtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Client is one connection to the node. Its methods match the session that
// internal/apply takes, so a test hands it to production code unchanged.
type Client struct {
	conn *ssh.Client
}

// Connect returns a client on a new in-memory connection to the server. The
// test fails if the connection cannot be made.
func (s *Server) Connect() *Client {
	s.t.Helper()
	client, err := s.dial(s.login)
	if err != nil {
		s.t.Fatalf("sshtest: connect: %v", err)
	}
	s.t.Cleanup(func() { client.Close() })
	return &Client{conn: client}
}

// dial connects with the key of the caller, so a test can offer one that the
// server does not list.
func (s *Server) dial(signer ssh.Signer) (*ssh.Client, error) {
	hostSigner, _, err := keyPair()
	if err != nil {
		return nil, err
	}

	clientConn, serverConn := memPipe()
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(s.login.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("the server does not list this key")
		},
	}
	cfg.AddHostKey(hostSigner)
	go s.serve(serverConn, cfg)
	s.track(serverConn)

	cc, chans, reqs, err := ssh.NewClientConn(clientConn, "pipe:22", &ssh.ClientConfig{
		User:            "meshy",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKey(hostSigner),
		Timeout:         Timeout,
	})
	if err != nil {
		clientConn.Close()
		return nil, err
	}
	return ssh.NewClient(cc, chans, reqs), nil
}

func (s *Server) track(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns = append(s.conns, conn)
}

// Close ends the connection.
func (c *Client) Close() error { return c.conn.Close() }

// Run runs one command and returns its output and status. A command that exits
// non-zero is a result, not an error.
func (c *Client) Run(ctx context.Context, cmd string) (node.Result, error) {
	return c.run(ctx, cmd, nil)
}

// Write puts data at path with mode. The bytes travel on the session's stdin,
// as they do in internal/sshx, so the node needs no scp or sftp binary.
func (c *Client) Write(ctx context.Context, path string, data []byte, mode fs.FileMode) error {
	res, err := c.run(ctx, writeCommand(path, mode), bytes.NewReader(data))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("write %s: exit %d: %s", path, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// Read returns the bytes at path. A path that the node does not hold reports
// node.ErrNoFile.
func (c *Client) Read(ctx context.Context, path string) ([]byte, error) {
	res, err := c.Run(ctx, readCommand(path))
	if err != nil {
		return nil, err
	}
	switch res.ExitCode {
	case 0:
		return []byte(res.Stdout), nil
	case missingExit:
		return nil, node.ErrNoFile
	}
	return nil, fmt.Errorf("read %s: exit %d: %s", path, res.ExitCode, strings.TrimSpace(res.Stderr))
}

func (c *Client) run(ctx context.Context, cmd string, stdin io.Reader) (node.Result, error) {
	sess, err := c.conn.NewSession()
	if err != nil {
		return node.Result{}, fmt.Errorf("%s: %w", cmd, err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	var out, errOut bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &errOut
	if stdin != nil {
		sess.Stdin = stdin
	}

	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		return node.Result{}, fmt.Errorf("%s: %w", cmd, ctx.Err())
	case err := <-done:
		res := node.Result{Stdout: out.String(), Stderr: errOut.String()}
		if err != nil {
			var exit *ssh.ExitError
			if errors.As(err, &exit) {
				res.ExitCode = exit.ExitStatus()
				return res, nil
			}
			return res, fmt.Errorf("%s: %w", cmd, err)
		}
		return res, nil
	}
}

// writeCommand and readCommand repeat the wire format that internal/sshx owns.
// sshx keeps those builders unexported and asserts their text in its own tests,
// so the harness only has to serve what they send.
func writeCommand(path string, mode fs.FileMode) string {
	quoted := quote(path)
	return fmt.Sprintf("umask 077; cat > %s && chmod %04o %s", quoted, mode.Perm(), quoted)
}

func readCommand(path string) string {
	quoted := quote(path)
	return fmt.Sprintf("if [ ! -e %s ]; then exit %d; fi; cat %s", quoted, missingExit, quoted)
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
