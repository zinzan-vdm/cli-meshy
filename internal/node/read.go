package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

const (
	// ReadLimit is how many nodes are read at once.
	ReadLimit = 4
	// Timeout bounds one command on one node.
	Timeout = 10 * time.Second
)

// Result is what one command returned.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Executor runs one read-only command on one node.
type Executor interface {
	Run(ctx context.Context, cmd string) (Result, error)
}

// ErrNoFile reports a path that the node does not hold. A caller that writes
// files needs to tell a missing file from a failed read.
var ErrNoFile = errors.New("no such file")

// Dir is the config directory on a node.
func Dir(mesh string) string { return "/etc/nebula/meshy-" + mesh }

// Unit is the systemd unit that runs the mesh on a node.
func Unit(mesh string) string { return "nebula@meshy-" + mesh }

// Commands returns the read-only commands for one node, in order.
func Commands(m config.Mesh, n config.Node) []string {
	d := Dir(m.Name)
	return []string{
		"nebula -version",
		"systemctl is-active " + Unit(m.Name),
		"nebula-cert print -json -path " + d + "/" + n.Name + ".crt",
		"ip link show " + m.Interface,
		"sha256sum " + d + "/config.yml",
		"nebula-cert print -json -path " + d + "/ca.crt",
	}
}

// Read runs the read-only commands on one node and returns the state it finds.
func Read(ctx context.Context, m config.Mesh, n config.Node, ex Executor) (State, error) {
	cmds := Commands(m, n)

	run := func(i int) (Result, error) {
		c, cancel := context.WithTimeout(ctx, Timeout)
		defer cancel()
		res, err := ex.Run(c, cmds[i])
		if err != nil {
			return res, fmt.Errorf("node %s: %s: %w", n.Name, cmds[i], err)
		}
		return res, nil
	}
	fail := func(i int, res Result) error {
		return fmt.Errorf("node %s: %s: exit %d: %s", n.Name, cmds[i], res.ExitCode, strings.TrimSpace(res.Stderr))
	}

	res, err := run(0)
	if err != nil {
		return State{}, err
	}
	if res.ExitCode != 0 {
		return State{}, fail(0, res)
	}
	version, err := ParseVersion(res.Stdout)
	if err != nil {
		return State{}, fmt.Errorf("node %s: %w", n.Name, err)
	}

	// is-active exits non-zero when the unit is down, and that is a fact, not a failure.
	res, err = run(1)
	if err != nil {
		return State{}, err
	}
	service := ParseService(res.Stdout)

	res, err = run(2)
	if err != nil {
		return State{}, err
	}
	if res.ExitCode != 0 {
		return State{}, fmt.Errorf("node %s: no certificate at %s/%s.crt: %s", n.Name, Dir(m.Name), n.Name, strings.TrimSpace(res.Stderr))
	}
	cert, err := ParseCert(res.Stdout)
	if err != nil {
		return State{}, fmt.Errorf("node %s: %w", n.Name, err)
	}

	res, err = run(3)
	if err != nil {
		return State{}, err
	}
	present := res.ExitCode == 0

	res, err = run(4)
	if err != nil {
		return State{}, err
	}
	if res.ExitCode != 0 {
		return State{}, fail(4, res)
	}
	hash, err := ParseHash(res.Stdout)
	if err != nil {
		return State{}, fmt.Errorf("node %s: %w", n.Name, err)
	}

	res, err = run(5)
	if err != nil {
		return State{}, err
	}
	if res.ExitCode != 0 {
		return State{}, fmt.Errorf("node %s: no certificate authority at %s/ca.crt: %s", n.Name, Dir(m.Name), strings.TrimSpace(res.Stderr))
	}
	ca, err := ParseCert(res.Stdout)
	if err != nil {
		return State{}, fmt.Errorf("node %s: %w", n.Name, err)
	}

	return State{
		Name:       n.Name,
		Version:    version,
		Service:    service,
		Interface:  present,
		ConfigHash: hash,
		Cert:       cert,
		Authority:  ca,
	}, nil
}

// ReadAll reads every server node, four at a time. A node that fails is
// reported and does not stop the others.
func ReadAll(ctx context.Context, m config.Mesh, open func(config.Node) (Executor, error)) ([]State, []error) {
	var (
		mu     sync.Mutex
		states []State
		errs   []error
	)
	sem := make(chan struct{}, ReadLimit)
	var wg sync.WaitGroup

	for _, n := range m.Nodes {
		if n.Kind != config.KindServer {
			continue
		}
		wg.Add(1)
		go func(n config.Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ex, err := open(n)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("node %s: %w", n.Name, err))
				mu.Unlock()
				return
			}
			if c, ok := ex.(interface{ Close() error }); ok {
				defer c.Close()
			}
			st, err := Read(ctx, m, n, ex)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			states = append(states, st)
		}(n)
	}
	wg.Wait()
	return states, errs
}
