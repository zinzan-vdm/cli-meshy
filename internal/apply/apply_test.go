package apply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

type fakeFile struct {
	body []byte
	mode fs.FileMode
}

// fakeNode is one node with a filesystem in memory. It answers the commands
// that Run builds, so it cannot drift from the commands that Run sends.
type fakeNode struct {
	files     map[string]fakeFile
	noUnit    bool
	inactive  bool
	staysDown bool
	renameErr bool
	reloadErr bool
	staleHash bool
	writeErr  map[string]error
	readErr   map[string]error
	log       []string
	reloads   int
	restarts  int
}

func newFake() *fakeNode {
	return &fakeNode{files: map[string]fakeFile{}}
}

func (f *fakeNode) Run(_ context.Context, cmd string) (node.Result, error) {
	switch {
	case cmd == unitCommand("homelab"):
		f.log = append(f.log, "cat")
		if f.noUnit {
			return node.Result{ExitCode: 1, Stderr: "No files found for nebula@meshy-homelab.service."}, nil
		}
		return node.Result{}, nil
	case cmd == restartCommand("homelab"):
		f.log = append(f.log, "restart")
		f.restarts++
		if f.reloadErr {
			return node.Result{ExitCode: 1, Stderr: "Job for nebula@meshy-homelab.service failed."}, nil
		}
		if !f.staysDown {
			f.inactive = false
		}
		return node.Result{}, nil
	case cmd == reloadCommand("homelab"):
		f.log = append(f.log, "reload")
		f.reloads++
		if f.reloadErr {
			return node.Result{ExitCode: 1, Stderr: "Job for nebula@meshy-homelab.service failed."}, nil
		}
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "mv -f "):
		from, to := movePaths(cmd)
		f.log = append(f.log, "rename "+from+" "+to)
		if f.renameErr {
			return node.Result{ExitCode: 1, Stderr: "Permission denied"}, nil
		}
		file, ok := f.files[from]
		if !ok {
			return node.Result{ExitCode: 1, Stderr: "No such file or directory"}, nil
		}
		delete(f.files, from)
		f.files[to] = file
		return node.Result{}, nil
	case strings.HasPrefix(cmd, "sha256sum "):
		path := unquote(strings.TrimPrefix(cmd, "sha256sum "))
		f.log = append(f.log, "hash "+path)
		file, ok := f.files[path]
		if !ok {
			return node.Result{ExitCode: 1, Stderr: "No such file or directory"}, nil
		}
		if f.staleHash {
			file.body = []byte("what the node served before the write\n")
		}
		sum := sha256.Sum256(file.body)
		return node.Result{Stdout: hex.EncodeToString(sum[:]) + "  " + path + "\n"}, nil
	case cmd == activeCommand("homelab"):
		f.log = append(f.log, "active")
		if f.inactive {
			return node.Result{Stdout: "inactive\n", ExitCode: 3}, nil
		}
		return node.Result{Stdout: "active\n"}, nil
	}
	return node.Result{ExitCode: 127, Stderr: "command not found: " + cmd}, nil
}

func (f *fakeNode) Write(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	f.log = append(f.log, "write "+path)
	if err, ok := f.writeErr[path]; ok {
		return err
	}
	f.files[path] = fakeFile{body: append([]byte(nil), data...), mode: mode}
	return nil
}

func (f *fakeNode) Read(_ context.Context, path string) ([]byte, error) {
	f.log = append(f.log, "read "+path)
	if err, ok := f.readErr[path]; ok {
		return nil, err
	}
	file, ok := f.files[path]
	if !ok {
		return nil, node.ErrNoFile
	}
	return file.body, nil
}

func (f *fakeNode) indexOf(op string) int {
	for i, entry := range f.log {
		if entry == op {
			return i
		}
	}
	return -1
}

func (f *fakeNode) lastRename() int {
	last := -1
	for i, entry := range f.log {
		if strings.HasPrefix(entry, "rename ") {
			last = i
		}
	}
	return last
}

func movePaths(cmd string) (string, string) {
	parts := strings.Split(strings.TrimPrefix(cmd, "mv -f "), "' '")
	if len(parts) != 2 {
		return "", ""
	}
	return unquote(parts[0]), unquote(parts[1])
}

func unquote(s string) string { return strings.Trim(s, "'") }

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
		Config:    []byte("pki:\n  ca: /etc/nebula/meshy-homelab/ca.crt\n"),
		Authority: []byte("authority certificate\n"),
		Cert:      []byte("node certificate\n"),
		Key:       []byte("node key\n"),
	}
}

func TestRunWritesEveryFileThroughATemporaryName(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	res, err := Run(context.Background(), m, n, testFiles(), f)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"config.yml", "ca.crt", "core-1.crt", "core-1.key"}
	if strings.Join(res.Writes, " ") != strings.Join(want, " ") {
		t.Fatalf("writes = %v, want %v", res.Writes, want)
	}

	dir := node.Dir(m.Name)
	for name, mode := range map[string]fs.FileMode{
		"config.yml": 0o600,
		"ca.crt":     0o644,
		"core-1.crt": 0o644,
		"core-1.key": 0o600,
	} {
		path := filepath.Join(dir, name)
		file, ok := f.files[path]
		if !ok {
			t.Fatalf("%s is not on the node", path)
		}
		if file.mode != mode {
			t.Errorf("%s mode = %o, want %o", name, file.mode, mode)
		}
		if f.indexOf("write "+path+".tmp") < 0 {
			t.Errorf("%s was not written through a temporary name", name)
		}
		if f.indexOf("rename "+path+".tmp "+path) < 0 {
			t.Errorf("%s was not renamed into place", name)
		}
	}

	for path := range f.files {
		if strings.HasSuffix(path, ".tmp") {
			t.Errorf("%s was left behind", path)
		}
	}
	if f.reloads != 1 {
		t.Errorf("the unit reloaded %d times, want 1", f.reloads)
	}
	if f.indexOf("reload") < f.lastRename() {
		t.Error("the unit reloaded before the last write")
	}
	if res.Action != ActionReload {
		t.Errorf("action = %q, want %q", res.Action, ActionReload)
	}
}

func TestRunReadsTheHashAndTheUnitBackAfterTheReload(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	if _, err := Run(context.Background(), m, n, testFiles(), f); err != nil {
		t.Fatalf("Run: %v", err)
	}

	last := f.log[len(f.log)-2:]
	if last[0] != "hash "+node.Dir(m.Name)+"/config.yml" {
		t.Errorf("the run did not read the config hash back: %v", last)
	}
	if last[1] != "active" {
		t.Errorf("the run did not read the unit state back: %v", last)
	}
}

func TestRunWritesNothingWhenTheNodeMatches(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	if _, err := Run(context.Background(), m, n, testFiles(), f); err != nil {
		t.Fatalf("the first Run failed: %v", err)
	}

	f.log = nil
	f.reloads = 0
	res, err := Run(context.Background(), m, n, testFiles(), f)
	if err != nil {
		t.Fatalf("the second Run failed: %v", err)
	}
	if len(res.Writes) != 0 || res.Action != ActionNone {
		t.Errorf("the second run reported %v, action %q", res.Writes, res.Action)
	}
	if f.reloads != 0 {
		t.Errorf("the unit reloaded %d times with no write", f.reloads)
	}
	for _, op := range f.log {
		if strings.HasPrefix(op, "write ") || strings.HasPrefix(op, "rename ") {
			t.Errorf("the second run touched the node: %s", op)
		}
	}
}

func TestRunStartsAUnitThatIsNotRunning(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.inactive = true

	res, err := Run(context.Background(), m, n, testFiles(), f)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if f.restarts != 1 {
		t.Errorf("the unit started %d times, want 1", f.restarts)
	}
	if f.reloads != 0 {
		t.Errorf("the unit reloaded %d times, want 0", f.reloads)
	}
	if res.Action != ActionRestart {
		t.Errorf("action = %q, want %q", res.Action, ActionRestart)
	}
	if f.indexOf("restart") < f.lastRename() {
		t.Error("the unit started before the last write")
	}
}

func TestRunWritesOnlyTheFileThatDiffers(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	if _, err := Run(context.Background(), m, n, testFiles(), f); err != nil {
		t.Fatalf("the first Run failed: %v", err)
	}

	path := node.Dir(m.Name) + "/config.yml"
	f.files[path] = fakeFile{body: []byte("pki: the old config\n"), mode: 0o600}
	f.log = nil
	f.reloads = 0

	res, err := Run(context.Background(), m, n, testFiles(), f)
	if err != nil {
		t.Fatalf("the second Run failed: %v", err)
	}
	if strings.Join(res.Writes, " ") != "config.yml" {
		t.Errorf("writes = %v, want only config.yml", res.Writes)
	}
	if f.reloads != 1 {
		t.Errorf("the unit reloaded %d times, want 1", f.reloads)
	}
	if string(f.files[path].body) != string(testFiles().Config) {
		t.Error("the node does not hold the rendered config")
	}
}

func TestRunNamesBootstrapWhenTheUnitIsMissing(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.noUnit = true

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded with no unit on the node")
	}
	if !strings.Contains(err.Error(), "run meshy bootstrap core-1") {
		t.Errorf("the error does not name the bootstrap command: %v", err)
	}
	for _, op := range f.log {
		if strings.HasPrefix(op, "write ") {
			t.Errorf("the run wrote to a node with no unit: %s", op)
		}
	}
}

func TestRunLeavesTheOldConfigWhenTheRenameFails(t *testing.T) {
	m, n := testMesh()
	f := newFake()

	path := node.Dir(m.Name) + "/config.yml"
	f.files[path] = fakeFile{body: []byte("pki: the old config\n"), mode: 0o600}
	f.renameErr = true

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the rename failed")
	}
	if string(f.files[path].body) != "pki: the old config\n" {
		t.Errorf("the failed rename changed the config: %q", f.files[path].body)
	}
	if f.reloads != 0 {
		t.Error("the unit reloaded after a failed rename")
	}
}

func TestRunStopsWhenTheWriteFails(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	temp := node.Dir(m.Name) + "/config.yml.tmp"
	f.writeErr = map[string]error{temp: errors.New("no space left on device")}

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the write failed")
	}
	if !strings.Contains(err.Error(), "no space left on device") {
		t.Errorf("the error does not name the reason: %v", err)
	}
	if f.indexOf("rename "+temp+" "+strings.TrimSuffix(temp, ".tmp")) >= 0 {
		t.Error("the run renamed a file that it could not write")
	}
	if f.reloads != 0 {
		t.Error("the unit reloaded after a failed write")
	}
}

func TestRunNamesTheUnitWhenTheReloadFails(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.reloadErr = true

	res, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the reload failed")
	}
	if !strings.Contains(err.Error(), unitName(m.Name)) {
		t.Errorf("the error does not name the unit: %v", err)
	}
	if len(res.Writes) != 4 {
		t.Errorf("writes = %v, want the four files that reached the node", res.Writes)
	}
	if _, ok := f.files[node.Dir(m.Name)+"/config.yml"]; !ok {
		t.Error("the written config left the node")
	}
}

func TestRunReportsAWriteThatTheNodeDidNotKeep(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.staleHash = true

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the node kept the old config")
	}
	if !strings.Contains(err.Error(), "the mesh renders") {
		t.Errorf("the error does not report the hash that the node holds: %v", err)
	}
}

func TestRunReportsAUnitThatIsNotActive(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.inactive = true
	f.staysDown = true

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the unit is not active")
	}
	if !strings.Contains(err.Error(), "is not active") {
		t.Errorf("the error does not report the unit state: %v", err)
	}
}

func TestRunStopsWhenAFileCannotBeRead(t *testing.T) {
	m, n := testMesh()
	f := newFake()
	f.readErr = map[string]error{node.Dir(m.Name) + "/config.yml": errors.New("permission denied")}

	_, err := Run(context.Background(), m, n, testFiles(), f)
	if err == nil {
		t.Fatal("Run succeeded although the config could not be read")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("the error does not name the reason: %v", err)
	}
	if f.reloads != 0 {
		t.Error("the unit reloaded after a failed read")
	}
}
