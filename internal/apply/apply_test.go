package apply

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/sshx/sshtest"
)

func testMesh() (config.Mesh, config.Node) {
	n := config.Node{
		Name: "core-1",
		Kind: config.KindServer,
		IP:   "10.42.0.1",
		SSH:  &config.SSH{User: "meshy", Host: "203.0.113.10"},
	}
	m := config.Mesh{Name: "homelab", Interface: "meshy0", Nodes: []config.Node{n}}
	return m, n
}

func testFiles() Files {
	return Files{
		Blocklist: []byte("aa11bb22\n"),
		Config:    []byte("pki:\n  ca: /etc/nebula/meshy-homelab/ca.crt\n"),
		Authority: []byte("authority certificate\n"),
		Cert:      []byte("node certificate\n"),
		Key:       []byte("node key\n"),
	}
}

// opsAfter returns the operations of one kind that the node did after seq.
func opsAfter(s *sshtest.Server, seq int, kind string) []sshtest.Op {
	var out []sshtest.Op
	for _, op := range s.Ops() {
		if op.Seq > seq && op.Kind == kind {
			out = append(out, op)
		}
	}
	return out
}

// cmdsAfter returns the commands that the node ran after seq.
func cmdsAfter(s *sshtest.Server, seq int) []string {
	var out []string
	for _, c := range s.Commands() {
		if c.Seq > seq {
			out = append(out, c.Text)
		}
	}
	return out
}

func commandSeq(s *sshtest.Server, text string) int {
	for _, c := range s.Commands() {
		if c.Text == text {
			return c.Seq
		}
	}
	return -1
}

func countCommand(s *sshtest.Server, text string) int {
	n := 0
	for _, c := range s.Commands() {
		if c.Text == text {
			n++
		}
	}
	return n
}

func lastRenameSeq(s *sshtest.Server) int {
	last := -1
	for _, op := range s.Ops() {
		if op.Kind == sshtest.OpRename {
			last = op.Seq
		}
	}
	return last
}

func renamed(s *sshtest.Server, from, to string) bool {
	for _, op := range s.Ops() {
		if op.Kind == sshtest.OpRename && op.Path == from && op.To == to {
			return true
		}
	}
	return false
}

func TestRunWritesEveryFileThroughATemporaryName(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)

	res, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"blocklist.txt", "config.yml", "ca.crt", "core-1.crt", "core-1.key"}
	if strings.Join(res.Writes, " ") != strings.Join(want, " ") {
		t.Fatalf("writes = %v, want %v", res.Writes, want)
	}

	dir := node.Dir(m.Name)
	written := opsAfter(srv, 0, sshtest.OpWrite)
	for name, mode := range map[string]fs.FileMode{
		"blocklist.txt": 0o644,
		"config.yml":    0o600,
		"ca.crt":        0o644,
		"core-1.crt":    0o644,
		"core-1.key":    0o600,
	} {
		path := dir + "/" + name
		f, ok := srv.File(path)
		if !ok {
			t.Fatalf("%s is not on the node", path)
		}
		if f.Mode != mode {
			t.Errorf("%s mode = %o, want %o", name, f.Mode, mode)
		}
		wrote := false
		for _, op := range written {
			if op.Path == path+".tmp" {
				wrote = true
			}
		}
		if !wrote {
			t.Errorf("%s was not written through a temporary name", name)
		}
		if !renamed(srv, path+".tmp", path) {
			t.Errorf("%s was not renamed into place", name)
		}
	}

	for _, path := range srv.Paths() {
		if strings.HasSuffix(path, ".tmp") {
			t.Errorf("%s was left behind", path)
		}
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 1 {
		t.Errorf("the unit reloaded %d times, want 1", got)
	}
	if commandSeq(srv, reloadCommand(m.Name)) < lastRenameSeq(srv) {
		t.Error("the unit reloaded before the last write")
	}
	if res.Action != ActionReload {
		t.Errorf("action = %q, want %q", res.Action, ActionReload)
	}
}

func TestRunReadsTheHashAndTheUnitBackAfterTheReload(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)

	if _, err := Run(context.Background(), m, n, testFiles(), srv.Connect()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	cmds := srv.Commands()
	last := cmds[len(cmds)-2:]
	if last[0].Text != hashCommand(node.Dir(m.Name)+"/config.yml") {
		t.Errorf("the run did not read the config hash back: %v", last)
	}
	if last[1].Text != activeCommand(m.Name) {
		t.Errorf("the run did not read the unit state back: %v", last)
	}
}

func TestRunWritesNothingWhenTheNodeMatches(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	c := srv.Connect()

	if _, err := Run(context.Background(), m, n, testFiles(), c); err != nil {
		t.Fatalf("the first Run failed: %v", err)
	}

	mark := srv.Seq()
	res, err := Run(context.Background(), m, n, testFiles(), c)
	if err != nil {
		t.Fatalf("the second Run failed: %v", err)
	}
	if len(res.Writes) != 0 || res.Action != ActionNone {
		t.Errorf("the second run reported %v, action %q", res.Writes, res.Action)
	}
	if ops := opsAfter(srv, mark, sshtest.OpWrite); len(ops) != 0 {
		t.Errorf("the second run wrote %v", ops)
	}
	if ops := opsAfter(srv, mark, sshtest.OpRename); len(ops) != 0 {
		t.Errorf("the second run renamed %v", ops)
	}
	for _, cmd := range cmdsAfter(srv, mark) {
		if cmd == reloadCommand(m.Name) || cmd == restartCommand(m.Name) {
			t.Errorf("the second run refreshed the unit: %q", cmd)
		}
	}
}

func TestRunStartsAUnitThatIsNotRunning(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	srv.StopUnit()

	res, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := countCommand(srv, restartCommand(m.Name)); got != 1 {
		t.Errorf("the unit started %d times, want 1", got)
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 0 {
		t.Errorf("the unit reloaded %d times, want 0", got)
	}
	if res.Action != ActionRestart {
		t.Errorf("action = %q, want %q", res.Action, ActionRestart)
	}
	if commandSeq(srv, restartCommand(m.Name)) < lastRenameSeq(srv) {
		t.Error("the unit started before the last write")
	}
}

func TestRunWritesOnlyTheFileThatDiffers(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	c := srv.Connect()

	if _, err := Run(context.Background(), m, n, testFiles(), c); err != nil {
		t.Fatalf("the first Run failed: %v", err)
	}

	path := node.Dir(m.Name) + "/config.yml"
	srv.Put(path, []byte("pki: the old config\n"), 0o600)
	mark := srv.Seq()

	res, err := Run(context.Background(), m, n, testFiles(), c)
	if err != nil {
		t.Fatalf("the second Run failed: %v", err)
	}
	if strings.Join(res.Writes, " ") != "config.yml" {
		t.Errorf("writes = %v, want only config.yml", res.Writes)
	}
	wrote := opsAfter(srv, mark, sshtest.OpWrite)
	if len(wrote) != 1 || wrote[0].Path != path+".tmp" {
		t.Errorf("the run wrote %v, want only %s", wrote, path+".tmp")
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 2 {
		t.Errorf("the unit reloaded %d times, want one per run", got)
	}
	if f, ok := srv.File(path); !ok || string(f.Body) != string(testFiles().Config) {
		t.Error("the node does not hold the rendered config")
	}
}

func TestRunNamesBootstrapWhenTheUnitIsMissing(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	srv.FailCommand(unitCommand(m.Name), 1, "No files found for "+unitName(m.Name)+".")

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded with no unit on the node")
	}
	if !strings.Contains(err.Error(), "run meshy bootstrap core-1") {
		t.Errorf("the error does not name the bootstrap command: %v", err)
	}
	if ops := opsAfter(srv, 0, sshtest.OpWrite); len(ops) != 0 {
		t.Errorf("the run wrote to a node with no unit: %v", ops)
	}
}

func TestRunLeavesTheOldConfigWhenTheRenameFails(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)

	path := node.Dir(m.Name) + "/config.yml"
	srv.Put(path, []byte("pki: the old config\n"), 0o600)
	srv.FailCommand(moveCommand(path+".tmp", path), 1, "Permission denied")

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the rename failed")
	}
	f, ok := srv.File(path)
	if !ok || string(f.Body) != "pki: the old config\n" {
		t.Errorf("the failed rename changed the config: %q", f.Body)
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 0 {
		t.Error("the unit reloaded after a failed rename")
	}
}

func TestRunStopsWhenTheWriteFails(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	temp := node.Dir(m.Name) + "/config.yml.tmp"
	srv.FailWrite(temp, "no space left on device")

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the write failed")
	}
	if !strings.Contains(err.Error(), "no space left on device") {
		t.Errorf("the error does not name the reason: %v", err)
	}
	if renamed(srv, temp, strings.TrimSuffix(temp, ".tmp")) {
		t.Error("the run renamed a file that it could not write")
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 0 {
		t.Error("the unit reloaded after a failed write")
	}
}

func TestRunNamesTheUnitWhenTheReloadFails(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	srv.FailCommand(reloadCommand(m.Name), 1, "Job for "+unitName(m.Name)+" failed.")

	res, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the reload failed")
	}
	if !strings.Contains(err.Error(), unitName(m.Name)) {
		t.Errorf("the error does not name the unit: %v", err)
	}
	if len(res.Writes) != 5 {
		t.Errorf("writes = %v, want the five files that reached the node", res.Writes)
	}
	if _, ok := srv.File(node.Dir(m.Name) + "/config.yml"); !ok {
		t.Error("the written config left the node")
	}
}

func TestRunReportsAWriteThatTheNodeDidNotKeep(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)

	path := node.Dir(m.Name) + "/config.yml"
	srv.Put(path, []byte("pki: the old config\n"), 0o600)
	srv.KeepRename(path)

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the node kept the old config")
	}
	if !strings.Contains(err.Error(), "the mesh renders") {
		t.Errorf("the error does not report the hash that the node holds: %v", err)
	}
}

func TestRunReportsAUnitThatIsNotActive(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	srv.StopUnit()
	srv.LeaveUnitDown()

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the unit is not active")
	}
	if !strings.Contains(err.Error(), "is not active") {
		t.Errorf("the error does not report the unit state: %v", err)
	}
}

func TestRunStopsWhenAFileCannotBeRead(t *testing.T) {
	m, n := testMesh()
	srv := sshtest.New(t)
	srv.FailRead(node.Dir(m.Name)+"/config.yml", "permission denied")

	_, err := Run(context.Background(), m, n, testFiles(), srv.Connect())
	if err == nil {
		t.Fatal("Run succeeded although the config could not be read")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("the error does not name the reason: %v", err)
	}
	if got := countCommand(srv, reloadCommand(m.Name)); got != 0 {
		t.Error("the unit reloaded after a failed read")
	}
}
