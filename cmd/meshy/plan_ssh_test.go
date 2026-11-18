package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
)

type testServer struct {
	port       int
	knownHosts string
	answers    map[string]string
	stop       func()
}

func startTestServer(t *testing.T) *testServer {
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

	srv := &testServer{answers: map[string]string{}, stop: func() { ln.Close() }}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	srv.port, _ = strconv.Atoi(port)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTestConn(conn, cfg, srv)
		}
	}()

	block, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("MESHY_SSH_KEY", keyPath)

	line := "[127.0.0.1]:" + strconv.Itoa(srv.port) + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostSigner.PublicKey()))) + "\n"
	srv.knownHosts = filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(srv.knownHosts, []byte(line), 0o600); err != nil {
		t.Fatalf("write known hosts: %v", err)
	}
	t.Setenv("MESHY_KNOWN_HOSTS", srv.knownHosts)

	return srv
}

func serveTestConn(conn net.Conn, cfg *ssh.ServerConfig, srv *testServer) {
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
				ch.Write([]byte(srv.answers[payload.Command]))
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{0}))
				return
			}
		}()
	}
}

// writeMesh writes a mesh file that names one server node on the test server.
func writeMesh(t *testing.T, srv *testServer) string {
	t.Helper()
	dir := t.TempDir()
	body := fmt.Sprintf(`schema: 1
name: homelab
cidr: 10.42.0.0/24
interface: meshy0
nebula:
  version: v1.12.0
lighthouses:
  - node: core-1
    port: 4242
nodes:
  - name: core-1
    kind: server
    groups: [core]
    ip: 10.42.0.1
    public_addr: 203.0.113.10:4242
    ssh:
      user: meshy
      host: 127.0.0.1
      port: %d
`, srv.port)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

// answerFacts fills the server answers with what a matching node returns.
func answerFacts(t *testing.T, dir string, srv *testServer, version string) {
	t.Helper()
	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rendered, err := nebula.Render(*mesh, mesh.Nodes[0])
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	sum := sha256.Sum256(rendered)

	srv.answers = map[string]string{
		"nebula -version":                          "Version: " + version + "\n",
		"systemctl is-active nebula@meshy-homelab": "active\n",
		"nebula-cert print -json -path /etc/nebula/meshy-homelab/core-1.crt": `{"details":{"name":"core-1","groups":["core"],"ips":["10.42.0.1/24"],"notAfter":"2035-01-01T00:00:00Z"},"fingerprint":"aa11"}`,
		"ip link show meshy0":                                            "2: meshy0: <POINTOPOINT>\n",
		"sha256sum /etc/nebula/meshy-homelab/config.yml":                 hex.EncodeToString(sum[:]) + "  config.yml\n",
		"nebula-cert print -json -path /etc/nebula/meshy-homelab/ca.crt": `{"details":{"name":"ca-1","isCA":true},"fingerprint":"ffee"}`,
	}
}

func capture(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	code := fn()

	w.Close()
	os.Stdout = old
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(body), code
}

func TestPlanExitsZeroWhenTheNodeMatches(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.12.0")

	body, code := capture(t, func() int { return cmdPlan([]string{"--mesh", dir}) })
	if code != 0 {
		t.Errorf("exit = %d, want 0: %s", code, body)
	}
	if body != "core-1  ok\n" {
		t.Errorf("text = %q", body)
	}
}

func TestPlanExitsOneWhenTheNodeDiffers(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.11.0")

	body, code := capture(t, func() int { return cmdPlan([]string{"--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1: %s", code, body)
	}
	if body != "core-1  binary  runs v1.11.0, pinned v1.12.0\n" {
		t.Errorf("text = %q", body)
	}
}

func TestPlanPrintsJSONAndNothingElse(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.11.0")

	body, code := capture(t, func() int { return cmdPlan([]string{"--json", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1: %s", code, body)
	}

	want := `{
  "mesh": "homelab",
  "nodes": [
    {
      "name": "core-1",
      "changes": [
        {
          "kind": "binary",
          "detail": "runs v1.11.0, pinned v1.12.0"
        }
      ]
    }
  ],
  "changes": 1
}
`
	if body != want {
		t.Errorf("json =\n%s\nwant\n%s", body, want)
	}
}

func TestPlanRefusesAValidMeshWithoutKnownHosts(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.12.0")
	t.Setenv("MESHY_KNOWN_HOSTS", "")

	body, code := capture(t, func() int { return cmdPlan([]string{"--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if body != "" {
		t.Errorf("stdout = %q, want nothing", body)
	}
}

func TestPlanReadsNoNodeWhenTheMeshIsInvalid(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.12.0")
	body, _ := os.ReadFile(filepath.Join(dir, config.FileName))
	broken := strings.Replace(string(body), "name: homelab", "name: Bad_Name", 1)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(broken), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}

	out, code := capture(t, func() int { return cmdPlan([]string{"--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
}

// answerBundle replaces the bundle that the node presents, so the plan reads an
// authority with a known life.
func answerBundle(srv *testServer, body string) {
	srv.answers["nebula-cert print -json -path /etc/nebula/meshy-homelab/ca.crt"] = body
}

const bundleWithDates = `{"details":{"name":"ca-1","isCA":true,"notBefore":"2026-01-01T00:00:00Z","notAfter":"2046-01-01T00:00:00Z"},"fingerprint":"ffee"}`

// The age comes from the bundle that a node presents, so the plan needs no
// vault. A workflow reads this text and the exit code stays untouched.
func TestPlanPrintsTheAgeOfTheAuthority(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.12.0")
	answerBundle(srv, bundleWithDates)

	body, code := capture(t, func() int { return cmdPlan([]string{"--mesh", dir}) })
	if code != 0 {
		t.Errorf("exit = %d, want 0: %s", code, body)
	}
	if !strings.HasPrefix(body, "mesh homelab\n  authority  ca-1 (") {
		t.Errorf("text =\n%s\nwant the age block above the node rows", body)
	}
	if !strings.HasSuffix(body, " of 20y)\n  due        no\n  next       phase 1 from 2042-01-01\n\ncore-1  ok\n") {
		t.Errorf("text =\n%s\nwant the due state, the phase date and the node row", body)
	}
}

func TestPlanPrintsTheRotationAsJSON(t *testing.T) {
	srv := startTestServer(t)
	defer srv.stop()

	dir := writeMesh(t, srv)
	answerFacts(t, dir, srv, "1.12.0")
	answerBundle(srv, bundleWithDates)

	body, code := capture(t, func() int { return cmdPlan([]string{"--json", "--mesh", dir}) })
	if code != 0 {
		t.Errorf("exit = %d, want 0: %s", code, body)
	}
	for _, want := range []string{
		`"rotation": {`,
		`"authority": "ca-1",`,
		`"not_before": "2026-01-01T00:00:00Z",`,
		`"rotate_at": 80,`,
		`"next": "phase 1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("json does not hold %q:\n%s", want, body)
		}
	}
}
