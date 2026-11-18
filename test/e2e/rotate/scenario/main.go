// Command scenario runs the live rotation of MESHY-034 and MESHY-035. It
// creates a vault in the run, applies the mesh to two live containers, rotates
// the trust anchor in three phases, and pings across the tunnel while phase 2
// replaces the certificates and phase 3 drops the old authority. Each
// assertion prints one line.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// dirName is where the CLI writes the files of the mesh on a node.
const dirName = "/etc/nebula/meshy-rot"

// peer is the overlay address that the ping crosses the tunnel to reach.
const peer = "10.42.0.1"

func main() {
	os.Exit(run(os.Args[1:]))
}

type cluster struct {
	meshy      string
	compose    string
	meshDir    string
	key        string
	knownHosts string
	certTool   string
}

func run(args []string) int {
	flags := flag.NewFlagSet("scenario", flag.ContinueOnError)
	c := &cluster{}
	flags.StringVar(&c.meshy, "meshy", "", "path to the meshy binary")
	flags.StringVar(&c.compose, "compose", "test/e2e/rotate/compose.yml", "path to the compose file")
	flags.StringVar(&c.meshDir, "mesh", "", "directory for the throwaway mesh")
	flags.StringVar(&c.key, "key", "", "ssh private key")
	flags.StringVar(&c.knownHosts, "known-hosts", "", "known hosts file")
	flags.StringVar(&c.certTool, "nebula-cert", "", "path to the pinned nebula-cert")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	for _, held := range []struct{ name, value string }{
		{"-meshy", c.meshy},
		{"-mesh", c.meshDir},
		{"-key", c.key},
		{"-known-hosts", c.knownHosts},
		{"-nebula-cert", c.certTool},
	} {
		if held.value == "" {
			fmt.Fprintf(os.Stderr, "scenario: %s is required\n", held.name)
			return 2
		}
	}

	if err := c.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "rotate: %v\n", err)
		return 1
	}
	fmt.Println("rotate: 10 assertions passed")
	return 0
}

// rotate walks the scenario. Every step that passes prints one line.
func (c *cluster) rotate() error {
	if err := c.createVault(); err != nil {
		return err
	}
	pass(1, "the vault is sealed and unlocked with shares from this run")

	if _, err := c.meshyRun("apply", "--mesh", c.meshDir); err != nil {
		return fmt.Errorf("apply: %w", err)
	}
	if err := c.waitForPing(); err != nil {
		return err
	}
	pass(2, "both nodes hold a certificate and the tunnel carries a ping to %s", peer)

	before, err := c.certOf("node-a", "node-a")
	if err != nil {
		return err
	}
	beforeB, err := c.certOf("node-b", "node-b")
	if err != nil {
		return err
	}
	pass(3, "node-a signs with %s and node-b signs with %s before the rotation", before.Issuer, beforeB.Issuer)

	if _, err := c.meshyRun("ca", "rotate", "--phase", "1", "--mesh", c.meshDir); err != nil {
		return fmt.Errorf("phase 1: %w", err)
	}
	if err := c.waitForTrust("node-a"); err != nil {
		return err
	}
	if err := c.waitForTrust("node-b"); err != nil {
		return err
	}
	pass(4, "phase 1 gives both nodes the trust bundle of ca-1 and ca-2")

	// Both nodes still hold a certificate that ca-1 signed, so phase 3 must
	// refuse and must leave the mesh file alone.
	refused, err := c.meshyRun("ca", "rotate", "--phase", "3", "--mesh", c.meshDir)
	if err == nil {
		return fmt.Errorf("phase 3 ran while node-a still held a certificate from ca-1")
	}
	if !strings.Contains(refused, "still holds a certificate from") {
		return fmt.Errorf("phase 3 refused without naming the node and its authority: %s", strings.TrimSpace(refused))
	}
	pass(5, "phase 3 refuses while a node still holds a certificate from ca-1")

	// The ping runs through phase 2, so a reload that drops the tunnel would
	// show as a lost reply.
	if err := c.startPing(); err != nil {
		return fmt.Errorf("start the ping: %w", err)
	}
	// The reload must happen inside a window that already carries traffic, so
	// the count speaks about the reload and not about a ping that began late.
	if err := c.waitForReplies(3); err != nil {
		return err
	}
	if _, err := c.meshyRun("ca", "rotate", "--phase", "2", "--mesh", c.meshDir); err != nil {
		c.stopPing()
		return fmt.Errorf("phase 2: %w", err)
	}
	sent, got, err := c.stopPing()
	if err != nil {
		return err
	}

	after, err := c.certOf("node-a", "node-a")
	if err != nil {
		return err
	}
	afterB, err := c.certOf("node-b", "node-b")
	if err != nil {
		return err
	}
	if after.Fingerprint == before.Fingerprint {
		return fmt.Errorf("node-a holds the certificate %s before and after phase 2", after.Fingerprint)
	}
	if afterB.Fingerprint == beforeB.Fingerprint {
		return fmt.Errorf("node-b holds the certificate %s before and after phase 2", afterB.Fingerprint)
	}
	pass(6, "phase 2 replaces the certificate of node-a (%s -> %s) and of node-b (%s -> %s)",
		short(before.Fingerprint), short(after.Fingerprint), short(beforeB.Fingerprint), short(afterB.Fingerprint))

	// Phase 2 reloads both nodes in about a second, and the ping runs at 0.1
	// seconds, so five replies cover that window. The count guards against a
	// ping that never carried traffic. The loss check carries the assertion.
	if sent < 5 {
		return fmt.Errorf("the ping sent %d replies, too few to cover the reload", sent)
	}
	if got != sent {
		return fmt.Errorf("the tunnel lost %d of %d replies during phase 2", sent-got, sent)
	}
	pass(7, "the tunnel lost 0 of %d replies while phase 2 reloaded both nodes", sent)

	second, err := c.authority("node-a", "ca-2")
	if err != nil {
		return err
	}
	for _, held := range []struct {
		name string
		cert node.Cert
	}{{"node-a", after}, {"node-b", afterB}} {
		if !strings.EqualFold(held.cert.Issuer, second.Fingerprint) {
			return fmt.Errorf("%s reports issuer %s, ca-2 is %s", held.name, held.cert.Issuer, second.Fingerprint)
		}
	}
	pass(8, "every node signs with ca-2 (%s)", short(second.Fingerprint))

	// Phase 3 drops the authority while the tunnel carries traffic, so a node
	// that loses its trust would show as a lost reply.
	if err := c.startPing(); err != nil {
		return fmt.Errorf("start the ping: %w", err)
	}
	if err := c.waitForReplies(3); err != nil {
		return err
	}
	phase3, err := c.meshyRun("ca", "rotate", "--phase", "3", "--mesh", c.meshDir)
	if err != nil {
		c.stopPing()
		return fmt.Errorf("phase 3: %w", err)
	}
	sent3, got3, err := c.stopPing()
	if err != nil {
		return err
	}
	if sent3 < 5 {
		return fmt.Errorf("the ping sent %d replies, too few to cover phase 3", sent3)
	}
	if got3 != sent3 {
		return fmt.Errorf("the tunnel lost %d of %d replies during phase 3", sent3-got3, sent3)
	}
	if !strings.Contains(phase3, "the rotation is complete") {
		return fmt.Errorf("phase 3 did not report a complete rotation: %s", strings.TrimSpace(phase3))
	}
	pass(9, "phase 3 removes ca-1 and the tunnel lost 0 of %d replies", sent3)

	if err := c.waitForOneAuthority("node-a"); err != nil {
		return err
	}
	if err := c.waitForOneAuthority("node-b"); err != nil {
		return err
	}
	last, err := c.authority("node-a", "ca-2")
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(c.meshDir, vault.DirName, "ca-1.key")); err != nil {
		return fmt.Errorf("the vault dropped ca-1.key: %w", err)
	}
	pass(10, "both nodes trust ca-2 alone (%s) and ca-1.key stays in the vault", short(last.Fingerprint))
	return nil
}

// createVault seals a vault with shares that this run generates and opens it.
// No share comes from the repository.
func (c *cluster) createVault() error {
	out, err := c.meshyRun("ca", "init", "--mesh", c.meshDir)
	if err != nil {
		return fmt.Errorf("ca init: %w", err)
	}
	var shares []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "meshy-") {
			shares = append(shares, line)
		}
	}
	if len(shares) < 3 {
		return fmt.Errorf("ca init printed %d shares, want 5", len(shares))
	}
	for i, share := range shares[:3] {
		os.Setenv(fmt.Sprintf("MESHY_SHARE_%d", i+1), share)
	}
	if _, err := c.meshyRun("unlock", "--from-env", "--mesh", c.meshDir); err != nil {
		return fmt.Errorf("unlock: %w", err)
	}
	return nil
}

// waitForPing waits for the tunnel to carry one echo request.
func (c *cluster) waitForPing() error {
	var last string
	for i := 0; i < 120; i++ {
		out, err := c.composeRun("node-b", "ping", "-n", "-c", "1", "-W", "2", peer)
		if err == nil {
			return nil
		}
		last = out
		sleep()
	}
	return fmt.Errorf("node-b cannot ping %s: %s", peer, strings.TrimSpace(last))
}

// waitForTrust waits for the bundle on a node to hold both authorities.
func (c *cluster) waitForTrust(service string) error {
	for i := 0; i < 30; i++ {
		certs, err := c.authorities(service)
		if err == nil && len(certs) >= 2 {
			return nil
		}
		sleep()
	}
	return fmt.Errorf("%s does not hold two authorities", service)
}

// waitForOneAuthority waits for the bundle on a node to hold one authority,
// because the reload of the bundle comes after the file is written.
func (c *cluster) waitForOneAuthority(service string) error {
	for i := 0; i < 30; i++ {
		certs, err := c.authorities(service)
		if err == nil && len(certs) == 1 {
			return nil
		}
		sleep()
	}
	return fmt.Errorf("%s does not hold one authority", service)
}

func (c *cluster) startPing() error {
	_, err := c.composeRun("node-b", "sh", "-c",
		// Phase 2 reloads two nodes in about a second, so the interval must be
		// short enough for the window to hold the samples that the assertion
		// names.
		"rm -f /tmp/rotate-ping.log /tmp/rotate-ping.pid; ping -i 0.1 -W 1 "+peer+" >/tmp/rotate-ping.log 2>&1 & echo $! >/tmp/rotate-ping.pid")
	return err
}

// waitForReplies waits until the running ping has carried n replies. It counts
// the lines of the log, because a ping that is still running prints no summary.
func (c *cluster) waitForReplies(n int) error {
	for i := 0; i < 60; i++ {
		out, err := c.composeRun("node-b", "sh", "-c", "cat /tmp/rotate-ping.log 2>/dev/null || true")
		if err == nil && strings.Count(out, "bytes from") >= n {
			return nil
		}
		sleep()
	}
	return fmt.Errorf("the ping did not carry %d replies before the rotation", n)
}

// stopPing stops the ping and returns the replies that it sent and received.
func (c *cluster) stopPing() (int, int, error) {
	out, err := c.composeRun("node-b", "sh", "-c",
		"kill -INT $(cat /tmp/rotate-ping.pid) 2>/dev/null; sleep 1; cat /tmp/rotate-ping.log")
	if err != nil {
		return 0, 0, err
	}
	return parsePing(out)
}

func (c *cluster) certOf(service, name string) (node.Cert, error) {
	out, err := c.composeRun(service, "nebula-cert", "print", "-json", "-path", dirName+"/"+name+".crt")
	if err != nil {
		return node.Cert{}, err
	}
	return node.ParseCert(out)
}

func (c *cluster) authorities(service string) ([]node.Cert, error) {
	out, err := c.composeRun(service, "nebula-cert", "print", "-json", "-path", dirName+"/ca.crt")
	if err != nil {
		return nil, err
	}
	return node.ParseCerts(out)
}

// authority returns the trusted authority that the bundle on a node names.
func (c *cluster) authority(service, name string) (node.Cert, error) {
	certs, err := c.authorities(service)
	if err != nil {
		return node.Cert{}, err
	}
	for _, cert := range certs {
		if cert.Name == name {
			return cert, nil
		}
	}
	return node.Cert{}, fmt.Errorf("the bundle on %s does not hold %s", service, name)
}

func (c *cluster) meshyRun(args ...string) (string, error) {
	cmd := exec.Command(c.meshy, args...)
	cmd.Env = append(os.Environ(),
		"MESHY_KNOWN_HOSTS="+c.knownHosts,
		"MESHY_SSH_KEY="+c.key,
		"PATH="+filepath.Dir(c.certTool)+":"+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("meshy %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c *cluster) composeRun(service string, args ...string) (string, error) {
	full := append([]string{"compose", "-f", c.compose, "exec", "-T", service}, args...)
	out, err := exec.Command("docker", full...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker %s: %v: %s", strings.Join(full, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func pass(n int, format string, args ...any) {
	fmt.Printf("ok %d: "+format+"\n", append([]any{n}, args...)...)
}

// short keeps the first eight characters of a fingerprint, so a report line
// stays readable and still names the certificate.
func short(fingerprint string) string {
	if len(fingerprint) <= 8 {
		return fingerprint
	}
	return fingerprint[:8]
}
