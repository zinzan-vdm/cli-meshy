package bootstrap

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Binary is the path of the pinned Nebula release on a server node. The
// installer in MESHY-021 writes the file here, and the unit reads it.
const Binary = "/usr/local/bin/nebula"

// Script returns the shell script that prepares one server node. The caller
// prints it. This package reads no file, opens no connection, and never runs
// the result.
func Script(mesh config.Mesh, n config.Node) (string, error) {
	if n.Kind != config.KindServer {
		return "", fmt.Errorf("%s is a user node, run meshy bundle", n.Name)
	}
	if n.SSH == nil || n.SSH.User == "" {
		return "", fmt.Errorf("node %s has no ssh.user, so the script cannot name a user", n.Name)
	}

	unit := node.Unit(mesh.Name)
	values := struct {
		Mesh     string
		Node     string
		User     string
		Unit     string
		Dir      string
		UnitFile string
		Sudoers  string
		Binary   string
	}{
		Mesh:     mesh.Name,
		Node:     n.Name,
		User:     n.SSH.User,
		Unit:     unit,
		Dir:      node.Dir(mesh.Name),
		UnitFile: "/etc/systemd/system/" + unit + ".service",
		Sudoers:  "/etc/sudoers.d/meshy-" + mesh.Name,
		Binary:   Binary,
	}

	var buf strings.Builder
	if err := scriptTemplate.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("render the script for %s: %w", n.Name, err)
	}
	return buf.String(), nil
}

var scriptTemplate = template.Must(template.New("bootstrap").Funcs(template.FuncMap{"q": quote}).Parse(`#!/bin/sh
# meshy bootstrap for node {{.Node}} in mesh {{.Mesh}}.
# This script changes a host, so a person runs it with sudo. A second run
# changes nothing.
set -eu

USER_NAME={{q .User}}
UNIT={{q .Unit}}
DIR={{q .Dir}}
UNIT_FILE={{q .UnitFile}}
SUDOERS={{q .Sudoers}}

if [ "$(id -u)" -ne 0 ]; then
	echo "meshy bootstrap: run this script as root, for example: meshy bootstrap {{.Node}} | sudo sh" >&2
	exit 1
fi

for tool in systemctl useradd visudo stat; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "meshy bootstrap: $tool is not on PATH, so this host cannot become a node" >&2
		exit 1
	fi
done

# sudo matches a full command path, so the rule holds the path this host
# resolves rather than a path from another distribution.
SYSTEMCTL=$(command -v systemctl)

SUDOERS_TMP="$SUDOERS.meshy-tmp"
UNIT_TMP="$UNIT_FILE.meshy-tmp"
trap 'rm -f "$SUDOERS_TMP" "$UNIT_TMP"' EXIT

if ! id -u "$USER_NAME" >/dev/null 2>&1; then
	useradd --create-home --shell /usr/sbin/nologin "$USER_NAME"
fi

mkdir -p "$DIR"
chmod 0700 "$DIR"
if [ "$(stat -c %U "$DIR")" != "$USER_NAME" ]; then
	chown "$USER_NAME:$USER_NAME" "$DIR"
fi

# apply runs systemctl as the node user through sudo, so the rule lists the
# two verbs it uses and nothing else. See docs/adr/0009.
printf '%s\n' "# meshy bootstrap for node {{.Node}}. The node user reloads and restarts one unit." > "$SUDOERS_TMP"
printf '%s\n' "$USER_NAME ALL=(root) NOPASSWD: $SYSTEMCTL reload $UNIT.service, $SYSTEMCTL restart $UNIT.service" >> "$SUDOERS_TMP"
chmod 0440 "$SUDOERS_TMP"
visudo -cf "$SUDOERS_TMP" >/dev/null
if cmp -s "$SUDOERS_TMP" "$SUDOERS"; then
	rm -f "$SUDOERS_TMP"
else
	mv "$SUDOERS_TMP" "$SUDOERS"
fi

cat > "$UNIT_TMP" <<'MESHY_UNIT'
[Unit]
Description=Nebula for mesh {{.Mesh}}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.Binary}} -config {{.Dir}}/config.yml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
User={{.User}}
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
MESHY_UNIT

if [ -f "$UNIT_FILE" ] && cmp -s "$UNIT_TMP" "$UNIT_FILE"; then
	rm -f "$UNIT_TMP"
else
	mv "$UNIT_TMP" "$UNIT_FILE"
	chmod 0644 "$UNIT_FILE"
	systemctl daemon-reload
	systemctl enable "$UNIT" >/dev/null
	if systemctl is-active --quiet "$UNIT"; then
		systemctl restart "$UNIT"
	fi
fi

echo "node {{.Node}} is prepared for mesh {{.Mesh}}"
`))

// quote makes a value safe to paste into a single quoted shell word. A mesh
// name and a node name hold no quote, but ssh.user is not validated.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
