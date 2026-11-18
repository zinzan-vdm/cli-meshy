package node

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// replyFor answers one read command from a fixed table.
func replyFor(cmd string) Result {
	switch {
	case strings.HasPrefix(cmd, "nebula -version"):
		return Result{Stdout: "Version: 1.12.0\n"}
	case strings.HasPrefix(cmd, "systemctl is-active"):
		return Result{Stdout: "active\n"}
	case strings.HasPrefix(cmd, "nebula-cert print"):
		return Result{Stdout: certJSON}
	case strings.HasPrefix(cmd, "ip link show"):
		return Result{Stdout: "2: meshy0: <POINTOPOINT>\n"}
	case strings.HasPrefix(cmd, "sha256sum"):
		return Result{Stdout: strings.Repeat("ab", 32) + "  config.yml\n"}
	}
	return Result{ExitCode: 127, Stderr: "command not found"}
}

// fakeNode answers the read commands, with room to fail one of them.
type fakeNode struct {
	errs     map[string]error
	seen     []string
	failCert bool
}

func (f *fakeNode) Run(_ context.Context, cmd string) (Result, error) {
	f.seen = append(f.seen, cmd)
	if err, ok := f.errs[cmd]; ok {
		return Result{}, err
	}
	if f.failCert && strings.HasSuffix(cmd, "core-1.crt") {
		return Result{ExitCode: 1, Stderr: "open /etc/nebula/meshy-homelab/core-1.crt: no such file or directory"}, nil
	}
	return replyFor(cmd), nil
}

func testMesh() config.Mesh {
	return config.Mesh{
		Name:      "homelab",
		Interface: "meshy0",
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1", SSH: &config.SSH{User: "meshy", Host: "203.0.113.10"}},
		},
	}
}

func TestReadReturnsEveryFact(t *testing.T) {
	m := testMesh()
	f := &fakeNode{}

	st, err := Read(context.Background(), m, m.Nodes[0], f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if st.Name != "core-1" || st.Version != "1.12.0" || st.Service != "active" {
		t.Fatalf("unexpected state: %+v", st)
	}
	if !st.Interface {
		t.Error("interface reported absent")
	}
	if st.ConfigHash != strings.Repeat("ab", 32) {
		t.Errorf("config hash = %q", st.ConfigHash)
	}
	if !st.Cert.Present || st.Cert.Name != "core-1" {
		t.Errorf("certificate = %+v", st.Cert)
	}
	if len(st.Authorities) != 1 || !st.Authorities[0].Present {
		t.Errorf("authorities = %+v", st.Authorities)
	}
}

func TestReadAuthoritiesReturnsTheWholeBundle(t *testing.T) {
	m := testMesh()

	certs, err := ReadAuthorities(context.Background(), m, m.Nodes[0], bundleNode{})
	if err != nil {
		t.Fatalf("ReadAuthorities: %v", err)
	}
	if len(certs) != 2 {
		t.Fatalf("bundle holds %d authorities, want 2", len(certs))
	}
}

func TestReadAuthoritiesNamesTheNodeWhenTheFileIsAbsent(t *testing.T) {
	m := testMesh()

	_, err := ReadAuthorities(context.Background(), m, m.Nodes[0], missingNode{})
	if err == nil {
		t.Fatal("ReadAuthorities accepted a node with no ca.crt")
	}
	if !strings.Contains(err.Error(), "core-1") || !strings.Contains(err.Error(), "ca.crt") {
		t.Errorf("error does not name the node and the file: %v", err)
	}
}

// bundleNode answers the authority read with a two document bundle.
type bundleNode struct{}

func (bundleNode) Run(context.Context, string) (Result, error) {
	return Result{Stdout: bundleJSON}, nil
}

// missingNode answers the authority read as a node without ca.crt does.
type missingNode struct{}

func (missingNode) Run(context.Context, string) (Result, error) {
	return Result{ExitCode: 1, Stderr: "open /etc/nebula/meshy-homelab/ca.crt: no such file or directory"}, nil
}

func TestReadNamesTheNodeWhenItFails(t *testing.T) {
	m := testMesh()
	cmd := "nebula -version"
	f := &fakeNode{errs: map[string]error{cmd: errors.New("connection refused")}}

	_, err := Read(context.Background(), m, m.Nodes[0], f)
	if err == nil {
		t.Fatal("Read succeeded on an unreachable node")
	}
	if !strings.Contains(err.Error(), "core-1") || !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error does not name the node and the reason: %v", err)
	}
}

func TestReadSaysWhenThereIsNoCertificate(t *testing.T) {
	m := testMesh()
	f := &fakeNode{failCert: true}

	_, err := Read(context.Background(), m, m.Nodes[0], f)
	if err == nil {
		t.Fatal("Read succeeded with no certificate")
	}
	if !strings.Contains(err.Error(), "no certificate") || !strings.Contains(err.Error(), "core-1") {
		t.Errorf("error does not say the certificate is missing: %v", err)
	}
}

func TestCommandsAreReadOnly(t *testing.T) {
	m := testMesh()
	allowed := []string{"nebula -version", "systemctl is-active ", "nebula-cert print ", "ip link show ", "sha256sum "}
	for _, cmd := range Commands(m, m.Nodes[0]) {
		ok := false
		for _, p := range allowed {
			if strings.HasPrefix(cmd, p) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("command is not on the read-only list: %q", cmd)
		}
		for _, bad := range []string{"rm ", "mv ", "systemctl restart", "systemctl reload", "tee ", ">", "install "} {
			if strings.Contains(cmd, bad) {
				t.Errorf("command could change the node: %q", cmd)
			}
		}
	}
}

// countingNode tracks how many reads are in flight at once.
type countingNode struct {
	mu      sync.Mutex
	now     int
	max     int
	release time.Duration
}

func (c *countingNode) Run(ctx context.Context, cmd string) (Result, error) {
	c.mu.Lock()
	c.now++
	if c.now > c.max {
		c.max = c.now
	}
	c.mu.Unlock()

	select {
	case <-time.After(c.release):
	case <-ctx.Done():
	}

	c.mu.Lock()
	c.now--
	c.mu.Unlock()
	return replyFor(cmd), nil
}

func TestReadAllBoundsTheWorkInFlight(t *testing.T) {
	m := config.Mesh{Name: "homelab", Interface: "meshy0"}
	for i := 0; i < 12; i++ {
		m.Nodes = append(m.Nodes, config.Node{
			Name: string(rune('a'+i)) + "-node",
			Kind: config.KindServer,
			SSH:  &config.SSH{User: "meshy", Host: "203.0.113.10"},
		})
	}

	shared := &countingNode{release: 2 * time.Millisecond}
	states, errs := ReadAll(context.Background(), m, func(config.Node) (Executor, error) { return shared, nil })

	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(states) != 12 {
		t.Fatalf("read %d nodes, want 12", len(states))
	}
	if shared.max > ReadLimit {
		t.Errorf("%d reads ran at once, limit is %d", shared.max, ReadLimit)
	}
	if shared.max < 2 {
		t.Errorf("only %d read ran at once, the work is not parallel", shared.max)
	}
}

func TestReadAllSkipsUserNodes(t *testing.T) {
	m := config.Mesh{Name: "homelab", Interface: "meshy0", Nodes: []config.Node{
		{Name: "laptop", Kind: config.KindUser, IP: "10.42.0.50"},
	}}
	states, errs := ReadAll(context.Background(), m, func(config.Node) (Executor, error) {
		t.Error("opened a connection to a user node")
		return nil, nil
	})
	if len(states) != 0 || len(errs) != 0 {
		t.Errorf("states=%v errs=%v", states, errs)
	}
}

func TestReadAllReportsOneBadNodeAndKeepsTheRest(t *testing.T) {
	m := config.Mesh{Name: "homelab", Interface: "meshy0", Nodes: []config.Node{
		{Name: "good-1", Kind: config.KindServer, SSH: &config.SSH{User: "meshy", Host: "203.0.113.10"}},
		{Name: "bad-1", Kind: config.KindServer, SSH: &config.SSH{User: "meshy", Host: "203.0.113.11"}},
	}}
	states, errs := ReadAll(context.Background(), m, func(n config.Node) (Executor, error) {
		if n.Name == "bad-1" {
			return nil, errors.New("host key mismatch")
		}
		return &fakeNode{}, nil
	})
	if len(states) != 1 || states[0].Name != "good-1" {
		t.Errorf("states = %+v", states)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "bad-1") {
		t.Errorf("errs = %v", errs)
	}
}
