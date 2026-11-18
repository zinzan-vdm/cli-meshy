package install

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// fakeNode answers the commands that one install sends, so it cannot drift from
// the commands the package builds.
type fakeNode struct {
	goos       string
	goarch     string
	before     string
	after      string
	placeErr   bool
	restartErr bool
	log        []string
	writes     map[string]fs.FileMode
	placed     []byte
}

func newFake() *fakeNode {
	return &fakeNode{
		goos:   "Linux",
		goarch: "x86_64",
		before: "Version: 1.11.0\n",
		after:  "Version: 1.12.0\n",
		writes: map[string]fs.FileMode{},
	}
}

func (f *fakeNode) Run(_ context.Context, cmd string) (node.Result, error) {
	switch {
	case cmd == "uname -s":
		f.log = append(f.log, "uname-s")
		return node.Result{Stdout: f.goos + "\n"}, nil
	case cmd == "uname -m":
		f.log = append(f.log, "uname-m")
		return node.Result{Stdout: f.goarch + "\n"}, nil
	case cmd == "nebula -version":
		f.log = append(f.log, "version")
		// The release the node reports changes only after the binary is placed.
		if f.hasPlaced() {
			return node.Result{Stdout: f.after}, nil
		}
		if f.before == "" {
			return node.Result{ExitCode: 127, Stderr: "nebula: not found"}, nil
		}
		return node.Result{Stdout: f.before}, nil
	case strings.HasPrefix(cmd, "sudo -n install "):
		f.log = append(f.log, "place")
		if f.placeErr {
			return node.Result{ExitCode: 1, Stderr: "install: cannot create regular file: Permission denied"}, nil
		}
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "sudo -n systemctl restart "):
		f.log = append(f.log, "restart")
		if f.restartErr {
			return node.Result{ExitCode: 1, Stderr: "Job for the unit failed."}, nil
		}
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "rm -f "):
		f.log = append(f.log, "remove")
		return node.Result{}, nil
	}
	return node.Result{ExitCode: 127, Stderr: "command not found: " + cmd}, nil
}

func (f *fakeNode) Write(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	f.log = append(f.log, "write")
	f.writes[path] = mode
	f.placed = append([]byte(nil), data...)
	return nil
}

func (f *fakeNode) hasPlaced() bool { return len(f.placed) > 0 }

func (f *fakeNode) indexOf(op string) int {
	for i, entry := range f.log {
		if entry == op {
			return i
		}
	}
	return -1
}

func testMesh() (config.Mesh, config.Node) {
	n := config.Node{
		Name: "core-1",
		Kind: config.KindServer,
		IP:   "10.42.0.1",
		SSH:  &config.SSH{User: "meshy", Host: "203.0.113.10"},
	}
	m := config.Mesh{Name: "homelab", Interface: "meshy0", Nebula: config.Nebula{Version: "v1.12.0"}, Nodes: []config.Node{n}}
	return m, n
}

// binary returns a path that holds the bytes the install must send.
func binary(t *testing.T) func(context.Context, Platform) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nebula")
	if err := os.WriteFile(path, []byte("the nebula binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return func(context.Context, Platform) (string, error) { return path, nil }
}

func open(f *fakeNode) func(config.Node) (Session, error) {
	return func(config.Node) (Session, error) { return f, nil }
}

func TestRunMovesANodeThatRunsAnotherRelease(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	results := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	res := results[0]
	if res.Err != nil {
		t.Fatalf("Run: %v", res.Err)
	}
	if !res.Moved {
		t.Error("the result does not report the move")
	}
	if res.From != "1.11.0" {
		t.Errorf("from = %q, want 1.11.0", res.From)
	}

	if got, want := f.writes[node.Dir(m.Name)+"/"+Incoming], fs.FileMode(0o700); got != want {
		t.Errorf("the binary was written with mode %o, want %o", got, want)
	}
	if string(f.placed) != "the nebula binary" {
		t.Errorf("the node holds %q", f.placed)
	}
	if f.indexOf("place") < f.indexOf("write") {
		t.Error("the binary was placed before it was written")
	}
	if f.indexOf("restart") < f.indexOf("place") {
		t.Error("the unit restarted before the binary was placed")
	}
	if f.indexOf("remove") < f.indexOf("place") {
		t.Error("the incoming file was removed before it was placed")
	}
}

func TestRunLeavesANodeThatIsOnThePin(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.before = "Version: 1.12.0\n"

	results := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))
	if res := results[0]; res.Err != nil || res.Moved {
		t.Fatalf("result = %+v, want no move", res)
	}

	for _, op := range []string{"write", "place", "restart"} {
		if f.indexOf(op) >= 0 {
			t.Errorf("the run %s a node that is on the pin: %v", op, f.log)
		}
	}
}

func TestRunStartsFromANodeThatHasNoNebula(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.before = ""

	res := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))[0]
	if res.Err != nil {
		t.Fatalf("Run: %v", res.Err)
	}
	if !res.Moved {
		t.Error("the result does not report the move")
	}
	if res.From != "" {
		t.Errorf("from = %q, want nothing", res.From)
	}
}

func TestRunRefusesAPlatformWithNoAsset(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.goarch = "riscv64"

	res := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))[0]
	if res.Err == nil {
		t.Fatal("Run accepted a platform with no asset")
	}
	if !strings.Contains(res.Err.Error(), "runs linux/riscv64, which has no release asset") {
		t.Errorf("error = %v", res.Err)
	}
	if f.indexOf("write") >= 0 {
		t.Error("the run wrote to a node it cannot serve")
	}
}

func TestRunReportsAFailedPlaceAndContinues(t *testing.T) {
	m, first := testMesh()
	second := first
	second.Name = "core-2"
	second.SSH = &config.SSH{User: "meshy", Host: "203.0.113.11"}
	m.Nodes = []config.Node{first, second}

	bad := newFake()
	bad.placeErr = true
	good := newFake()
	nodes := map[string]*fakeNode{first.Name: bad, second.Name: good}
	openBoth := func(n config.Node) (Session, error) { return nodes[n.Name], nil }

	results := Run(context.Background(), m, m.Nodes, openBoth, binary(t))
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Err == nil {
		t.Error("the failed node reports no error")
	}
	if results[1].Err != nil || !results[1].Moved {
		t.Errorf("the second node did not move: %+v", results[1])
	}
	if bad.indexOf("restart") >= 0 {
		t.Error("the run restarted a node whose binary was not placed")
	}
	if good.indexOf("restart") < 0 {
		t.Error("the run did not restart the node it moved")
	}
}

func TestRunReportsAUnitThatDidNotRestart(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.restartErr = true

	res := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))[0]
	if res.Err == nil || !strings.Contains(res.Err.Error(), "systemctl restart") {
		t.Fatalf("error = %v, want the restart command", res.Err)
	}
	if res.Moved {
		t.Error("a node that did not restart reports a move")
	}
}

func TestRunReportsANodeThatKeepsTheOldRelease(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.after = "Version: 1.11.0\n"

	res := Run(context.Background(), m, []config.Node{n}, open(f), binary(t))[0]
	if res.Err == nil {
		t.Fatal("Run accepted a node that still runs the old release")
	}
	if !strings.Contains(res.Err.Error(), "reports \"1.11.0\" after the install") {
		t.Errorf("error = %v", res.Err)
	}
	if res.Moved {
		t.Error("a node that keeps the old release reports a move")
	}
}

func TestRunReportsANodeItCannotOpen(t *testing.T) {
	m, n := testMesh()
	open := func(config.Node) (Session, error) { return nil, errors.New("dial tcp: connection refused") }

	res := Run(context.Background(), m, []config.Node{n}, open, binary(t))[0]
	if res.Err == nil || !strings.Contains(res.Err.Error(), "node core-1: dial tcp") {
		t.Fatalf("error = %v", res.Err)
	}
}

func TestRunReportsACacheThatFails(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	fail := func(context.Context, Platform) (string, error) { return "", errors.New("no release v1.12.0") }

	res := Run(context.Background(), m, []config.Node{n}, open(f), fail)[0]
	if res.Err == nil || !strings.Contains(res.Err.Error(), "no release v1.12.0") {
		t.Fatalf("error = %v", res.Err)
	}
	if f.indexOf("write") >= 0 {
		t.Error("the run wrote to a node with no binary")
	}
}

func TestPlaceCommandNamesTheSourceTheOwnerAndTheMode(t *testing.T) {
	got := placeCommand("homelab")
	want := "sudo -n install -o root -g root -m 0755 '/etc/nebula/meshy-homelab/nebula.incoming' '/usr/local/bin/nebula'"
	if got != want {
		t.Errorf("placeCommand = %q, want %q", got, want)
	}
}

func TestPlatformOfReadsTheNode(t *testing.T) {
	for _, tc := range []struct {
		os, arch string
		want     Platform
	}{
		{"Linux", "x86_64", Platform{"linux", "amd64"}},
		{"Linux", "aarch64", Platform{"linux", "arm64"}},
		{"Darwin", "arm64", Platform{"darwin", "arm64"}},
	} {
		f := newFake()
		f.goos, f.goarch = tc.os, tc.arch

		got, err := PlatformOf(context.Background(), f)
		if err != nil {
			t.Fatalf("PlatformOf: %v", err)
		}
		if got != tc.want {
			t.Errorf("%s %s = %+v, want %+v", tc.os, tc.arch, got, tc.want)
		}
	}
}
