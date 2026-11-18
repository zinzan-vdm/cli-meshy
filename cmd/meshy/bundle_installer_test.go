package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/release"
)

// staged is a bundle unpacked into a directory, which is what a person does
// with the archive.
type staged struct {
	dir   string
	files map[string][]byte
}

func stage(t *testing.T, node string) staged {
	t.Helper()

	meshDir := writeApplyMesh(t)
	openApplyVault(t, meshDir, node)
	stubSigner(t)
	setExpiry(t, futureExpiry())
	cacheBinary(t, meshDir, "darwin", "arm64", []byte("PINNED BINARY"))

	if _, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", node, "--mesh", meshDir}) }); code != 0 {
		t.Fatalf("bundle failed with %d: %s", code, errOut)
	}

	files, modes := readArchive(t, archivePath(meshDir, node))
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, os.FileMode(modes[name])); err != nil {
			t.Fatalf("unpack %s: %v", name, err)
		}
	}
	return staged{dir: dir, files: files}
}

// tools writes the commands that the installer calls, so a test can run the
// installer with no root and no systemd. id answers the uid that the test
// chose, systemctl records its arguments, and ip reports that the interface is
// up.
func tools(t *testing.T, uid string) (bin, log string) {
	t.Helper()

	bin = t.TempDir()
	log = filepath.Join(t.TempDir(), "systemctl.log")
	scripts := map[string]string{
		"id":        "#!/bin/sh\necho " + uid + "\n",
		"systemctl": "#!/bin/sh\necho \"$@\" >> " + log + "\n",
		"ip":        "#!/bin/sh\nexit 0\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return bin, log
}

// TestLinuxInstallerPlacesTheFilesAndStartsTheUnit runs install.sh from a
// bundle. It moves every path under a prefix, so the test needs no root and
// touches no system file.
func TestLinuxInstallerPlacesTheFilesAndStartsTheUnit(t *testing.T) {
	held := stage(t, "laptop-1")
	bin, log := tools(t, "0")
	root := t.TempDir()

	cmd := exec.Command("/bin/sh", "install.sh")
	cmd.Dir = held.dir
	cmd.Env = envWith(bin, "MESHY_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v: %s", err, out)
	}

	if got := strings.TrimSpace(string(out)); got != "meshy-homelab is running" {
		t.Errorf("install.sh printed %q, want one line that says the mesh is running", got)
	}

	dir := filepath.Join(root, "etc", "nebula", "meshy-homelab")
	for name, want := range map[string]os.FileMode{
		"blocklist.txt": 0o644,
		"config.yml":    0o600,
		"ca.crt":        0o644,
		"laptop-1.crt":  0o644,
		"laptop-1.key":  0o600,
	} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s was not placed: %v", name, err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s has mode %o, want %o", name, info.Mode().Perm(), want)
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(body) != string(held.files[name]) {
			t.Errorf("%s differs from the copy in the archive", name)
		}
	}

	binary := filepath.Join(root, "usr", "local", "bin", release.Binary)
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("the binary was not placed: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("the binary has mode %o, want 0755", info.Mode().Perm())
	}

	unit := filepath.Join(root, "etc", "systemd", "system", "nebula@meshy-homelab.service")
	body, err := os.ReadFile(unit)
	if err != nil {
		t.Fatalf("the unit was not written: %v", err)
	}
	for _, want := range []string{
		"ExecStart=" + binary + " -config " + filepath.Join(dir, "config.yml"),
		"ExecReload=/bin/kill -HUP $MAINPID",
		"[Install]",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the unit does not hold %q:\n%s", want, body)
		}
	}

	commands, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("systemctl was not called: %v", err)
	}
	for _, want := range []string{
		"daemon-reload",
		"enable --now nebula@meshy-homelab.service",
	} {
		if !strings.Contains(string(commands), want) {
			t.Errorf("systemctl did not run %q:\n%s", want, commands)
		}
	}
}

// TestLinuxInstallerRefusesWithoutRoot runs the same script with a user id that
// is not root. The script must name the right it needs and write nothing.
func TestLinuxInstallerRefusesWithoutRoot(t *testing.T) {
	held := stage(t, "laptop-1")
	bin, _ := tools(t, "1000")
	root := t.TempDir()

	cmd := exec.Command("/bin/sh", "install.sh")
	cmd.Dir = held.dir
	cmd.Env = envWith(bin, "MESHY_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("install.sh exited zero without root")
	}
	if !strings.Contains(string(out), "run this installer as root") {
		t.Errorf("the message does not name the right: %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "etc")); err == nil {
		t.Error("a refused installer wrote to /etc")
	}
}

// TestMacInstallerRefusesWithoutRoot runs the guard of install.command. The
// rest of that script needs macOS, so a test cannot execute it here. The
// structure of the rest is asserted in internal/scaffold/installers.
func TestMacInstallerRefusesWithoutRoot(t *testing.T) {
	held := stage(t, "laptop-1")
	bin, _ := tools(t, "1000")

	cmd := exec.Command("/bin/sh", "install.command")
	cmd.Dir = held.dir
	cmd.Env = envWith(bin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("install.command exited zero without root")
	}
	if !strings.Contains(string(out), "run this installer as root") {
		t.Errorf("the message does not name the right: %q", out)
	}
	if !strings.Contains(string(out), "sudo") {
		t.Errorf("the message does not say how to get the right: %q", out)
	}
}

// systemPath is the directories that a Linux host keeps its tools in. The
// signing stub replaces PATH, so the test adds them back.
const systemPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// envWith replaces PATH, because a second PATH entry leaves the loader with
// the first one.
func envWith(bin string, extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra)+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PATH=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "PATH="+bin+":"+os.Getenv("PATH")+":"+systemPath)
	return append(env, extra...)
}

// futureExpiry is a date that no renewal window reaches.
func futureExpiry() time.Time { return time.Now().Add(300 * 24 * time.Hour) }
