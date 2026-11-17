package sshx

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Config is what a connection to one node needs.
type Config struct {
	User       string
	Host       string
	Port       int
	KeyPath    string
	KnownHosts string
	Timeout    time.Duration
}

// FromNode builds a Config from the mesh file and the environment.
func FromNode(n config.Node, knownHosts string) (Config, error) {
	if n.SSH == nil {
		return Config{}, fmt.Errorf("node %s has no ssh block", n.Name)
	}
	port := n.SSH.Port
	if port == 0 {
		port = 22
	}
	return Config{
		User:       n.SSH.User,
		Host:       n.SSH.Host,
		Port:       port,
		KeyPath:    os.Getenv("MESHY_SSH_KEY"),
		KnownHosts: knownHosts,
		Timeout:    node.Timeout,
	}, nil
}

// Client runs commands on one node over one connection.
type Client struct {
	client *ssh.Client
	host   string
}

// Dial opens one connection and checks the host key.
func Dial(cfg Config) (*Client, error) {
	auth, closers, err := authMethods(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()

	check, err := HostKeys(cfg.KnownHosts)
	if err != nil {
		return nil, err
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = node.Timeout
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", addr, err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: check,
		Timeout:         timeout,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s: %w", addr, err)
	}
	return &Client{client: ssh.NewClient(c, chans, reqs), host: addr}, nil
}

func authMethods(keyPath string) ([]ssh.AuthMethod, []net.Conn, error) {
	if keyPath != "" {
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("ssh key %s: %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("ssh key %s: %w", keyPath, err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil, nil
	}

	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, fmt.Errorf("no MESHY_SSH_KEY and no ssh agent, so there is no way to authenticate")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh agent %s: %w", sock, err)
	}
	ag := agent.NewClient(conn)
	return []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}, []net.Conn{conn}, nil
}

// Run runs one command and returns its output and exit status. A command that
// exits non-zero is a result, not an error.
func (c *Client) Run(ctx context.Context, cmd string) (node.Result, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return node.Result{}, fmt.Errorf("%s: %w", c.host, err)
	}
	defer sess.Close()

	var out, errOut bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &errOut

	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		return node.Result{}, fmt.Errorf("%s: %s: %w", c.host, cmd, ctx.Err())
	case err := <-done:
		res := node.Result{Stdout: out.String(), Stderr: errOut.String()}
		if err != nil {
			var exit *ssh.ExitError
			if ok := asExitError(err, &exit); ok {
				res.ExitCode = exit.ExitStatus()
				return res, nil
			}
			return res, fmt.Errorf("%s: %s: %w", c.host, cmd, err)
		}
		return res, nil
	}
}

func asExitError(err error, target **ssh.ExitError) bool {
	if e, ok := err.(*ssh.ExitError); ok {
		*target = e
		return true
	}
	return false
}

// Close closes the connection.
func (c *Client) Close() error { return c.client.Close() }
