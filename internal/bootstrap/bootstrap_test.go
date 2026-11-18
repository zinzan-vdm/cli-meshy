package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

func testMesh() config.Mesh {
	return config.Mesh{Name: "homelab"}
}

func serverNode() config.Node {
	return config.Node{
		Name: "web-1",
		Kind: config.KindServer,
		SSH:  &config.SSH{User: "meshy", Host: "203.0.113.11", Port: 22},
	}
}

func render(t *testing.T, mesh config.Mesh, n config.Node) string {
	t.Helper()

	body, err := Script(mesh, n)
	if err != nil {
		t.Fatalf("Script: %v", err)
	}
	return body
}

func TestScriptPreparesTheUserTheRuleTheDirectoryAndTheUnit(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	for _, want := range []string{
		"USER_NAME='meshy'",
		"DIR='/etc/nebula/meshy-homelab'",
		"UNIT='nebula@meshy-homelab'",
		"UNIT_FILE='/etc/systemd/system/nebula@meshy-homelab.service'",
		"SUDOERS='/etc/sudoers.d/meshy-homelab'",
		"chmod 0700 \"$DIR\"",
		"systemctl daemon-reload",
		"ExecStart=/usr/local/bin/nebula -config /etc/nebula/meshy-homelab/config.yml",
		"ExecReload=/bin/kill -HUP $MAINPID",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the script does not hold %q:\n%s", want, body)
		}
	}
}

func TestScriptNamesTheUnitThatApplyReloads(t *testing.T) {
	mesh := testMesh()
	body := render(t, mesh, serverNode())

	// The unit name comes from internal/node, which is the package that apply
	// reads, so the two names cannot drift.
	if want := node.Unit(mesh.Name); !strings.Contains(body, want+".service") {
		t.Fatalf("the script does not name %s.service", want)
	}
}

func TestScriptAllowsReloadAndRestartOfThatUnitOnly(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	rule := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "ALL=(root)") {
			rule = line
		}
	}
	if rule == "" {
		t.Fatalf("the script writes no sudoers rule:\n%s", body)
	}

	for _, want := range []string{
		"NOPASSWD:",
		"$SYSTEMCTL reload $UNIT.service",
		"$SYSTEMCTL restart $UNIT.service",
		"$INSTALL -o root -g root -m 0755 $DIR/nebula.incoming /usr/local/bin/nebula",
	} {
		if !strings.Contains(rule, want) {
			t.Errorf("the rule does not hold %q: %s", want, rule)
		}
	}
	if got := strings.Count(rule, "$SYSTEMCTL"); got != 2 {
		t.Errorf("the rule holds %d unit commands, want 2: %s", got, rule)
	}
	if got := strings.Count(rule, "$INSTALL"); got != 1 {
		t.Errorf("the rule holds %d install commands, want 1: %s", got, rule)
	}
	for _, other := range []string{" start", " stop", " status", " enable", " disable", "*"} {
		if strings.Contains(rule, other) {
			t.Errorf("the rule also allows %q: %s", other, rule)
		}
	}
}

func TestScriptPlacesTheBinaryThroughOneExactCommand(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	rule := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "ALL=(root)") {
			rule = line
		}
	}

	// The source, the destination, the owner and the mode are named, so the node
	// user cannot place another file or place this one as another owner.
	for _, want := range []string{
		"-o root -g root -m 0755",
		"$DIR/nebula.incoming",
		"/usr/local/bin/nebula",
	} {
		if !strings.Contains(rule, want) {
			t.Errorf("the rule does not hold %q: %s", want, rule)
		}
	}
	if strings.Contains(rule, "sudo sh") || strings.Contains(rule, "/bin/sh") {
		t.Errorf("the rule holds a shell: %s", rule)
	}
}

func TestScriptGivesTheNodeUserAShellThatRunsACommand(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	// sshd runs a command through the shell of the account, and it refuses the
	// command when the shell is nologin. This repository reaches every node as
	// this user, so the shell is part of the contract.
	line := ""
	for _, l := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "useradd") {
			line = trimmed
		}
	}
	if line == "" {
		t.Fatal("the script does not create the node user")
	}
	if strings.Contains(line, "nologin") {
		t.Errorf("the node user cannot run a command: %s", line)
	}
	if !strings.Contains(line, "-s /bin/sh") {
		t.Errorf("the node user holds no shell that runs a command: %s", line)
	}
}

func TestScriptRepairsAShellThatRefusesEveryCommand(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	// A host that ran an older script keeps a shell that denies every command.
	// The user exists, so the create block is skipped, and the repair carries
	// the host back to a shell that sshd will run.
	if !strings.Contains(body, `usermod -s /bin/sh "$USER_NAME"`) {
		t.Error("the script does not repair the shell of a node user that already exists")
	}
	if !strings.Contains(body, `SHELL_NOW=`) {
		t.Error("the script repairs the shell without reading the shell that the host holds")
	}
}

func TestScriptUnlocksTheAccountWithoutAPassword(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	// sshd refuses every method for a locked account, and useradd locks the
	// account. The script sets a field that no hash can match.
	if !strings.Contains(body, `usermod -p '*' "$USER_NAME"`) {
		t.Error("the script leaves the account locked, so sshd refuses the key")
	}
}

func TestScriptOwnsTheHomeDirectoryOfTheNodeUser(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	// sshd refuses an authorized key that sits in a home directory owned by
	// another user, so the script owns the directory that holds the key.
	if !strings.Contains(body, `chown "$USER_NAME:$USER_NAME" "$HOME_DIR"`) {
		t.Error("the script leaves the home directory to whoever created it")
	}
}

func TestScriptGuardsEveryChangeSoASecondRunDoesNothing(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	ordered := [][2]string{
		{`if ! id -u "$USER_NAME"`, `useradd -m -s /bin/sh`},
		{`if cmp -s "$SUDOERS_TMP" "$SUDOERS"`, `mv "$SUDOERS_TMP" "$SUDOERS"`},
		{`if [ -f "$UNIT_FILE" ] && cmp -s "$UNIT_TMP" "$UNIT_FILE"`, `mv "$UNIT_TMP" "$UNIT_FILE"`},
		{`mv "$UNIT_TMP" "$UNIT_FILE"`, `systemctl daemon-reload`},
		{`systemctl is-active --quiet "$UNIT"`, `systemctl restart "$UNIT"`},
	}
	for _, pair := range ordered {
		guard := strings.Index(body, pair[0])
		change := strings.Index(body, pair[1])
		if guard < 0 {
			t.Errorf("the script has no guard %q", pair[0])
			continue
		}
		if change < 0 {
			t.Errorf("the script has no change %q", pair[1])
			continue
		}
		if guard > change {
			t.Errorf("%q comes before its guard %q", pair[1], pair[0])
		}
	}
}

func TestScriptParsesAsAShellScript(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on this host")
	}

	cases := map[string]string{
		"a plain node":  render(t, testMesh(), serverNode()),
		"a quoted user": "",
	}
	odd := serverNode()
	odd.SSH.User = "meshy'admin user"
	cases["a quoted user"] = render(t, testMesh(), odd)

	for name, body := range cases {
		path := filepath.Join(t.TempDir(), "bootstrap.sh")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write script: %v", err)
		}
		// -n reads the script and stops before it runs. The script is never run.
		if out, err := exec.Command(shell, "-n", path).CombinedOutput(); err != nil {
			t.Errorf("%s does not parse: %v\n%s", name, err, out)
		}
	}
}

func TestScriptQuotesAnUnvalidatedUser(t *testing.T) {
	n := serverNode()
	n.SSH.User = "meshy'admin user"

	body := render(t, testMesh(), n)
	if !strings.Contains(body, `USER_NAME='meshy'\''admin user'`) {
		t.Errorf("the user is not quoted for the shell:\n%s", body)
	}
}

func TestScriptMakesNoNetworkConnection(t *testing.T) {
	body := render(t, testMesh(), serverNode())

	for _, command := range []string{"ssh ", "scp ", "curl", "wget", "git "} {
		if strings.Contains(body, command) {
			t.Errorf("the script reaches the network with %q:\n%s", command, body)
		}
	}
}

func TestScriptRefusesAUserNode(t *testing.T) {
	n := config.Node{Name: "horo-laptop", Kind: config.KindUser, Platform: "darwin/arm64"}

	_, err := Script(testMesh(), n)
	if err == nil {
		t.Fatal("a user node produced a script")
	}
	if want := "horo-laptop is a user node, run meshy bundle"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

func TestScriptRefusesAServerNodeWithoutSSH(t *testing.T) {
	n := config.Node{Name: "web-1", Kind: config.KindServer}

	_, err := Script(testMesh(), n)
	if err == nil {
		t.Fatal("a server node without ssh produced a script")
	}
	if !strings.Contains(err.Error(), "ssh.user") {
		t.Errorf("error = %q, want it to name ssh.user", err)
	}
}
