// Package install puts the pinned Nebula release on the server nodes.
package install

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/release"
)

// Session is one node, reachable for commands and for a file write.
type Session interface {
	node.Executor
	Write(ctx context.Context, path string, data []byte, mode fs.FileMode) error
}

// Binary is where the unit starts Nebula from, and the destination the sudoers
// rule names.
const Binary = "/usr/local/bin/nebula"

// Incoming is the name the binary travels under inside the node directory. The
// rule of MESHY-027 names this path, so it holds no shell and no wildcard.
const Incoming = "nebula.incoming"

// Platform is what a node runs, in the words the project uses for its archives.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string { return p.OS + "/" + p.Arch }

// Result is what one node did.
type Result struct {
	Node  string
	From  string
	Moved bool
	Err   error
}

// Run puts the pinned release on every target that runs another one. A node
// that fails is reported and the run continues.
func Run(ctx context.Context, m config.Mesh, targets []config.Node, open func(config.Node) (Session, error), binary func(context.Context, Platform) (string, error)) []Result {
	results := make([]Result, 0, len(targets))
	for _, n := range targets {
		results = append(results, one(ctx, m, n, open, binary))
	}
	return results
}

func one(ctx context.Context, m config.Mesh, n config.Node, open func(config.Node) (Session, error), binary func(context.Context, Platform) (string, error)) Result {
	res := Result{Node: n.Name}

	s, err := open(n)
	if err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}
	if c, ok := s.(interface{ Close() error }); ok {
		defer c.Close()
	}

	p, err := PlatformOf(ctx, s)
	if err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}
	if !release.Supported(p.OS, p.Arch) {
		res.Err = fmt.Errorf("node %s runs %s, which has no release asset", n.Name, p)
		return res
	}

	res.From = version(ctx, s)
	if release.Normalize(res.From) == release.Normalize(m.Nebula.Version) {
		return res
	}

	path, err := binary(ctx, p)
	if err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}
	body, err := os.ReadFile(path)
	if err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}

	incoming := node.Dir(m.Name) + "/" + Incoming
	if err := s.Write(ctx, incoming, body, 0o700); err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}
	// The binary is 22 MB, so the node keeps no copy of it.
	defer func() {
		_, _ = s.Run(context.WithoutCancel(ctx), removeCommand(incoming))
	}()

	if err := mustRun(ctx, s, placeCommand(m.Name)); err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}
	if err := mustRun(ctx, s, restartCommand(m.Name)); err != nil {
		res.Err = fmt.Errorf("node %s: %w", n.Name, err)
		return res
	}

	got := version(ctx, s)
	if release.Normalize(got) != release.Normalize(m.Nebula.Version) {
		res.Err = fmt.Errorf("node %s reports %q after the install, want %s", n.Name, got, m.Nebula.Version)
		return res
	}
	res.Moved = true
	return res
}

// PlatformOf asks the node what it runs.
func PlatformOf(ctx context.Context, s Session) (Platform, error) {
	goos, err := mustRunOut(ctx, s, "uname -s")
	if err != nil {
		return Platform{}, err
	}
	goarch, err := mustRunOut(ctx, s, "uname -m")
	if err != nil {
		return Platform{}, err
	}
	return Platform{OS: mapOS(goos), Arch: mapArch(goarch)}, nil
}

func mapOS(name string) string {
	switch name {
	case "Linux":
		return "linux"
	case "Darwin":
		return "darwin"
	}
	return strings.ToLower(name)
}

func mapArch(name string) string {
	switch name {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	}
	return name
}

// version reads the release a node runs. A node with no Nebula reports nothing,
// which is not an error: the install is what puts one there.
func version(ctx context.Context, s Session) string {
	res, err := s.Run(ctx, "nebula -version")
	if err != nil || res.ExitCode != 0 {
		return ""
	}
	got, err := node.ParseVersion(res.Stdout + res.Stderr)
	if err != nil {
		return ""
	}
	return got
}

func mustRun(ctx context.Context, s Session, cmd string) error {
	res, err := s.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%s: exit %d: %s", cmd, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

func mustRunOut(ctx context.Context, s Session, cmd string) (string, error) {
	res, err := s.Run(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("%s: exit %d: %s", cmd, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}

func placeCommand(mesh string) string {
	return "sudo -n install -o root -g root -m 0755 " + quote(node.Dir(mesh)+"/"+Incoming) + " " + quote(Binary)
}

func restartCommand(mesh string) string {
	return "sudo -n systemctl restart " + quote(node.Unit(mesh)+".service")
}

func removeCommand(path string) string { return "rm -f " + quote(path) }

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
