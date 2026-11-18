package apply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Session is one connection to a node that can read a file and write one.
// node.Executor stays the read-only contract, and this is the wider contract
// that a command which changes a node needs.
type Session interface {
	node.Executor
	Write(ctx context.Context, path string, data []byte, mode fs.FileMode) error
	Read(ctx context.Context, path string) ([]byte, error)
}

// Files is the material that one node must hold: the rendered config and the
// certificates that the vault holds for the node and its authority.
type Files struct {
	Config    []byte
	Authority []byte
	Cert      []byte
	Key       []byte
}

// Result is what one run changed on one node.
type Result struct {
	Writes []string
	Action Action
}

// Action is what a run did to the unit after the writes.
type Action string

const (
	ActionNone    Action = ""
	ActionReload  Action = "reload"
	ActionRestart Action = "restart"
)

// Run makes one node hold the files of the mesh. It writes only a file that
// differs, and it refreshes the unit once, after the last write.
func Run(ctx context.Context, m config.Mesh, n config.Node, f Files, s Session) (Result, error) {
	if err := checkUnit(ctx, m, n, s); err != nil {
		return Result{}, err
	}

	var res Result
	dir := node.Dir(m.Name)
	for _, want := range f.list(n) {
		wrote, err := replace(ctx, s, dir+"/"+want.name, want.body, want.mode)
		if err != nil {
			return res, fmt.Errorf("node %s: %w", n.Name, err)
		}
		if wrote {
			res.Writes = append(res.Writes, want.name)
		}
	}

	if len(res.Writes) > 0 {
		action, err := refresh(ctx, m, n, s)
		if err != nil {
			return res, err
		}
		res.Action = action
	}

	if err := readBack(ctx, m, n, f.Config, s); err != nil {
		return res, err
	}
	return res, nil
}

// refresh reloads the unit when it runs and starts it when it does not. A node
// that bootstrap prepared holds the unit enabled and stopped, and a reload
// cannot start a unit.
func refresh(ctx context.Context, m config.Mesh, n config.Node, s Session) (Action, error) {
	res, err := runCommand(ctx, s, activeCommand(m.Name))
	if err != nil {
		return ActionNone, fmt.Errorf("node %s: %w", n.Name, err)
	}

	active := strings.TrimSpace(res.Stdout) == "active"
	if active {
		return ActionReload, reload(ctx, m, n, s)
	}
	return ActionRestart, restart(ctx, m, n, s)
}

// replace puts body at path through a temporary name in the same directory, so
// a failed rename leaves the file that is already there. It reports whether it
// wrote.
func replace(ctx context.Context, s Session, path string, body []byte, mode fs.FileMode) (bool, error) {
	current, err := readFile(ctx, s, path)
	switch {
	case errors.Is(err, node.ErrNoFile):
	case err != nil:
		return false, fmt.Errorf("read %s: %w", path, err)
	case digest(current) == digest(body):
		return false, nil
	}

	temp := path + ".tmp"
	if err := writeFile(ctx, s, temp, body, mode); err != nil {
		return false, fmt.Errorf("write %s: %w", temp, err)
	}

	res, err := runCommand(ctx, s, moveCommand(temp, path))
	if err != nil {
		return false, fmt.Errorf("rename %s: %w", temp, err)
	}
	if res.ExitCode != 0 {
		return false, fmt.Errorf("rename %s: exit %d: %s", temp, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return true, nil
}

func checkUnit(ctx context.Context, m config.Mesh, n config.Node, s Session) error {
	res, err := runCommand(ctx, s, unitCommand(m.Name))
	if err != nil {
		return fmt.Errorf("node %s: %w", n.Name, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("node %s has no unit %s, run meshy bootstrap %s", n.Name, unitName(m.Name), n.Name)
	}
	return nil
}

func reload(ctx context.Context, m config.Mesh, n config.Node, s Session) error {
	res, err := runCommand(ctx, s, reloadCommand(m.Name))
	if err != nil {
		return fmt.Errorf("node %s: reload %s: %w", n.Name, unitName(m.Name), err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("node %s: reload %s: exit %d: %s", n.Name, unitName(m.Name), res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

func restart(ctx context.Context, m config.Mesh, n config.Node, s Session) error {
	res, err := runCommand(ctx, s, restartCommand(m.Name))
	if err != nil {
		return fmt.Errorf("node %s: start %s: %w", n.Name, unitName(m.Name), err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("node %s: start %s: exit %d: %s", n.Name, unitName(m.Name), res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// readBack compares the hash and the unit state that the node reports with what
// the mesh asks for, so a write that failed without an error is still visible.
func readBack(ctx context.Context, m config.Mesh, n config.Node, want []byte, s Session) error {
	path := node.Dir(m.Name) + "/config.yml"
	res, err := runCommand(ctx, s, hashCommand(path))
	if err != nil {
		return fmt.Errorf("node %s: %w", n.Name, err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("node %s: read back %s: exit %d: %s", n.Name, path, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	got, err := node.ParseHash(res.Stdout)
	if err != nil {
		return fmt.Errorf("node %s: %w", n.Name, err)
	}
	if got != digest(want) {
		return fmt.Errorf("node %s: %s holds %s, the mesh renders %s", n.Name, path, got, digest(want))
	}

	res, err = runCommand(ctx, s, activeCommand(m.Name))
	if err != nil {
		return fmt.Errorf("node %s: %w", n.Name, err)
	}
	if state := node.ParseService(res.Stdout); state != "active" {
		return fmt.Errorf("node %s: unit %s is not active: %s", n.Name, unitName(m.Name), state)
	}
	return nil
}

type file struct {
	name string
	mode fs.FileMode
	body []byte
}

// list returns the files of one node in the order the node takes them.
func (f Files) list(n config.Node) []file {
	return []file{
		{name: "config.yml", mode: 0o600, body: f.Config},
		{name: "ca.crt", mode: 0o644, body: f.Authority},
		{name: n.Name + ".crt", mode: 0o644, body: f.Cert},
		{name: n.Name + ".key", mode: 0o600, body: f.Key},
	}
}

// unitName is the full name of the unit. The sudoers rule that meshy bootstrap
// writes names the unit with its suffix, and sudo matches the whole argument.
func unitName(mesh string) string { return node.Unit(mesh) + ".service" }

func unitCommand(mesh string) string { return "systemctl cat " + quote(unitName(mesh)) }

func reloadCommand(mesh string) string { return "sudo -n systemctl reload " + quote(unitName(mesh)) }

func restartCommand(mesh string) string { return "sudo -n systemctl restart " + quote(unitName(mesh)) }

func activeCommand(mesh string) string { return "systemctl is-active " + quote(unitName(mesh)) }

func moveCommand(from, to string) string { return "mv -f " + quote(from) + " " + quote(to) }

func hashCommand(path string) string { return "sha256sum " + quote(path) }

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runCommand(ctx context.Context, s Session, cmd string) (node.Result, error) {
	c, cancel := context.WithTimeout(ctx, node.Timeout)
	defer cancel()
	return s.Run(c, cmd)
}

func readFile(ctx context.Context, s Session, path string) ([]byte, error) {
	c, cancel := context.WithTimeout(ctx, node.Timeout)
	defer cancel()
	return s.Read(c, path)
}

func writeFile(ctx context.Context, s Session, path string, body []byte, mode fs.FileMode) error {
	c, cancel := context.WithTimeout(ctx, node.Timeout)
	defer cancel()
	return s.Write(c, path, body, mode)
}
