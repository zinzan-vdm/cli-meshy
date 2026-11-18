#!/bin/sh
# The assertions of MESHY-043. This script runs in the runner container and
# drives meshy over ssh against a host that starts with no /etc/nebula content.
# It prints one line for each assertion that holds, and it stops at the first
# failure.
#
# Assertion 4 proves the join from the lighthouse, and not from the log of the
# host, so the ping leaves the network namespace of the lighthouse container.
set -eu

KEY=/work/ssh/id_ed25519
KNOWN=/work/known_hosts
MESH=/work/mesh
RELEASE=/work/release
LIGHTHOUSE=lighthouse
HOST=host
NODE=node-a
UNIT=nebula@meshy-hostlab.service
DIR=/etc/nebula/meshy-hostlab
MESHY=/usr/local/bin/meshy

export MESHY_SSH_KEY="$KEY"
export MESHY_KNOWN_HOSTS="$KNOWN"

PIN=$(awk '/^[^[:space:]#]/{b=$1} b=="nebula:" && /^[[:space:]]+version:/{print $2; exit}' "$MESH/mesh.yml" | tr -d '"')
if [ -z "$PIN" ]; then
	printf 'e2e: mesh.yml holds no nebula.version\n' >&2
	exit 1
fi

IP=$(awk -v want="$NODE" '
	/^  - name: / { name = $3 }
	name == want && /^    ip: / { print $2; exit }
' "$MESH/mesh.yml")
if [ -z "$IP" ]; then
	printf 'e2e: mesh.yml holds no ip for %s\n' "$NODE" >&2
	exit 1
fi

# The three paths that bootstrap owns. The checksum of this tree and the count of
# the recorded calls are what assertion 2 compares.
TREE='find /etc/nebula/meshy-hostlab /etc/sudoers.d/meshy-hostlab /etc/systemd/system/nebula@meshy-hostlab.service -type f -printf "%p %m %u %g\n" | sort; find /etc/nebula/meshy-hostlab /etc/sudoers.d/meshy-hostlab /etc/systemd/system/nebula@meshy-hostlab.service -type f -exec sha256sum {} + | sort'

fail() {
	printf 'e2e: %s\n' "$*" >&2
	exit 1
}

# on runs one command on one node. Every call checks the host key against the
# file that this script scanned.
on() {
	target=$1
	shift
	ssh -i "$KEY" -o "UserKnownHostsFile=$KNOWN" -o StrictHostKeyChecking=yes \
		-o BatchMode=yes -o ConnectTimeout=5 -o LogLevel=ERROR "$target" "$@"
}

scan() {
	name=$1
	tries=0
	until ssh-keyscan -T 2 -t ed25519 "$name" 2>/dev/null | grep -q .; do
		tries=$((tries + 1))
		[ "$tries" -ge 60 ] && fail "no sshd on $name after 60 tries"
		sleep 1
	done
	ssh-keyscan -T 2 -t ed25519 "$name" 2>/dev/null >>"$KNOWN"
}

wait_login() {
	target=$1
	tries=0
	until on "$target" true 2>/dev/null; do
		tries=$((tries + 1))
		[ "$tries" -ge 30 ] && fail "no key login for $target"
		sleep 1
	done
}

tree_sum() { on "root@$1" "{ $TREE; } | sha256sum"; }

calls() {
	on "root@$1" 'if [ -f /var/log/meshy-systemctl.log ]; then wc -l </var/log/meshy-systemctl.log; else printf "0\n"; fi'
}

binary_sum() {
	on "root@$1" 'if [ -x /usr/local/bin/nebula ]; then sha256sum /usr/local/bin/nebula | cut -d" " -f1; else printf "absent\n"; fi'
}

# bootstrap_node prints the script for one node and runs it as root over ssh.
# The node user then needs the key that the run places for it. The shell comes
# from the script, and the run checks that it runs a command rather than
# repairing it, because a repair would hide the defect that this run exists to
# catch.
bootstrap_node() {
	name=$1
	host=$2
	"$MESHY" bootstrap "$name" --mesh "$MESH" >"/work/$name.sh"
	[ -s "/work/$name.sh" ] || fail "bootstrap printed nothing for $name"
	on "root@$host" sh -s <"/work/$name.sh"

	on "root@$host" 'install -d -m 0700 -o meshy -g meshy /home/meshy/.ssh && install -m 0600 -o meshy -g meshy /run/keys/authorized_keys /home/meshy/.ssh/authorized_keys'

	if ! on "meshy@$host" true; then
		printf 'note: the account of the node user is %s\n' "$(on "root@$host" 'grep "^meshy:" /etc/passwd' || true)"
		on "root@$host" 'ls -ld /home/meshy /home/meshy/.ssh /home/meshy/.ssh/authorized_keys' || true
		printf 'note: the key the host holds is %s\n' "$(on "root@$host" 'ssh-keygen -lf /home/meshy/.ssh/authorized_keys' 2>&1 || true)"
		printf 'note: the key the runner holds is %s\n' "$(ssh-keygen -lf "$KEY.pub" 2>&1 || true)"
		on "root@$host" 'tail -n 20 /var/log/auth.log 2>/dev/null || true' || true
		fail "the node user cannot run a command on $host"
	fi
}

install_node() {
	"$MESHY" install --node "$1" --source "$RELEASE" --mesh "$MESH"
}

# The empty host is the first thing the run proves: it holds no /etc/nebula
# content and no binary at the path that install writes.
printf 'e2e: waiting for sshd on %s and %s\n' "$LIGHTHOUSE" "$HOST"
scan "$LIGHTHOUSE"
scan "$HOST"
wait_login "root@$LIGHTHOUSE"
wait_login "root@$HOST"

if on "root@$HOST" 'test -e /etc/nebula' 2>/dev/null; then
	fail "the host holds /etc/nebula before bootstrap"
fi
if on "root@$HOST" 'test -e /usr/local/bin/nebula' 2>/dev/null; then
	fail "the host holds a binary before install"
fi
printf 'note: the host holds no /etc/nebula content and no /usr/local/bin/nebula\n'

# The vault is created in this run. No share reaches the environment until
# assertion 4, so bootstrap and install run with the key sealed.
out=$("$MESHY" ca init --mesh "$MESH")
printf '%s\n' "$out" >/work/ca-init.txt
chmod 0600 /work/ca-init.txt
printf 'note: %s\n' "$(printf '%s\n' "$out" | sed -n '1p')"
shares=$(printf '%s\n' "$out" | grep '^  ' | sed 's/^  //')
count=$(printf '%s\n' "$shares" | grep -c . || true)
[ "$count" -eq 5 ] || fail "ca init printed $count shares, want 5"
if [ -e "$MESH/.meshy/age.identity" ]; then
	fail "the vault is open before bootstrap and install"
fi
printf 'note: the vault is sealed and holds no share in the environment\n'

# The mesh names the node, and the compose file names the host that serves it.
# They differ, so every call passes both.
bootstrap_node "$NODE" "$HOST"
printf 'note: the home of the node user is %s\n' "$(on "root@$HOST" 'ls -ld /home/meshy' 2>/dev/null || true)"

uid=$(on "root@$HOST" 'id -u meshy')
case "$uid" in
'' | *[!0-9]*) fail "the host has no meshy user" ;;
esac

held=$(on "root@$HOST" "stat -c '%a %U' $DIR")
[ "$held" = "700 meshy" ] || fail "$DIR is '$held', want '700 meshy'"

on "root@$HOST" "grep -qF 'meshy ALL=(root) NOPASSWD:' /etc/sudoers.d/meshy-hostlab" ||
	fail "the sudoers rule does not name the node user"
on "root@$HOST" "grep -qF 'reload $UNIT' /etc/sudoers.d/meshy-hostlab" ||
	fail "the sudoers rule does not allow reload of $UNIT"
on "root@$HOST" "grep -qF 'restart $UNIT' /etc/sudoers.d/meshy-hostlab" ||
	fail "the sudoers rule does not allow restart of $UNIT"
on "root@$HOST" "grep -qF -- '-o root -g root -m 0755 $DIR/nebula.incoming /usr/local/bin/nebula' /etc/sudoers.d/meshy-hostlab" ||
	fail "the sudoers rule does not name the install of the binary"
on "root@$HOST" 'visudo -cf /etc/sudoers.d/meshy-hostlab >/dev/null' ||
	fail "visudo rejects the sudoers rule"
held=$(on "root@$HOST" 'stat -c %a /etc/sudoers.d/meshy-hostlab')
[ "$held" = "440" ] || fail "the sudoers rule has mode $held, want 440"

held=$(on "root@$HOST" "stat -c %a /etc/systemd/system/$UNIT")
[ "$held" = "644" ] || fail "$UNIT has mode $held, want 644"
on "root@$HOST" "grep -qF 'ExecStart=/usr/local/bin/nebula -config $DIR/config.yml' /etc/systemd/system/$UNIT" ||
	fail "$UNIT does not name the binary and the config"
on "root@$HOST" "grep -qF 'User=meshy' /etc/systemd/system/$UNIT" ||
	fail "$UNIT does not run as the node user"
on "root@$HOST" "grep -qF 'Description=Nebula for mesh hostlab' /etc/systemd/system/$UNIT" ||
	fail "$UNIT does not name the mesh"
on "root@$HOST" "grep -qF 'WantedBy=multi-user.target' /etc/systemd/system/$UNIT" ||
	fail "$UNIT is not wanted by multi-user.target"

printf 'ok 1 bootstrap created the user, the sudoers rule, %s and %s\n' "$DIR" "$UNIT"

tree_before=$(tree_sum "$HOST")
calls_before=$(calls "$HOST")
on "root@$HOST" sh -s </work/host.sh
tree_after=$(tree_sum "$HOST")
calls_after=$(calls "$HOST")
[ "$tree_before" = "$tree_after" ] || fail "the second bootstrap changed the tree"
[ "$calls_before" = "$calls_after" ] || fail "the second bootstrap recorded $calls_after calls, was $calls_before"
printf 'ok 2 a second bootstrap changed nothing: tree %s, %s recorded calls\n' "$tree_before" "$calls_before"

install_node "$NODE"
want=$(awk '$2 == "nebula-linux-amd64.tar.gz/nebula" { print $1 }' "$RELEASE/SHASUM256.txt")
[ -n "$want" ] || fail "SHASUM256.txt names no member of the archive"
got=$(binary_sum "$HOST")
[ "$got" = "$want" ] || fail "the host holds $got, the pin is $want"
on "meshy@$HOST" '/usr/local/bin/nebula -version' | grep -qF "Version: ${PIN#v}" ||
	fail "the host does not report $PIN"
on "root@$HOST" "grep -qx 'restart $UNIT' /var/log/meshy-systemctl.log" ||
	fail "install did not restart $UNIT"
printf 'ok 3 install placed %s at /usr/local/bin/nebula and restarted %s\n' "$PIN" "$UNIT"

# The lighthouse is the peer of the join. It takes the same three commands and
# starts when apply writes its config. Its mesh name and its ssh host are the
# same word.
bootstrap_node "$LIGHTHOUSE" "$LIGHTHOUSE"
install_node "$LIGHTHOUSE"

i=0
for share in $shares; do
	i=$((i + 1))
	if [ "$i" -le 3 ]; then
		export "MESHY_SHARE_$i=$share"
	fi
done
# install fills .meshy/bin with the verified release, so the open of the vault
# replaces the record in a directory that already holds the tool cache.
out=$("$MESHY" unlock --from-env --force --mesh "$MESH")
printf 'note: the vault is open at %s\n' "$out"

"$MESHY" apply --node "$LIGHTHOUSE" --mesh "$MESH"
applied=$("$MESHY" apply --node "$NODE" --mesh "$MESH")
printf '%s\n' "$applied"
for file in blocklist.txt config.yml ca.crt "$NODE.crt" "$NODE.key"; do
	case "$applied" in
	*"$file"*) ;;
	*) fail "apply did not write $file" ;;
	esac
done
case "$applied" in
*started* | *reloaded*) ;;
*) fail "apply did not start the unit" ;;
esac
held=$(on "root@$HOST" "stat -c %a $DIR/$NODE.crt $DIR/$NODE.key | tr '\n' ' '")
[ "$held" = "644 600 " ] || fail "the certificate and the key have modes '$held', want '644 600 '"

tries=0
until on "root@$LIGHTHOUSE" "ping -n -c 1 -W 2 $IP" >/work/ping.txt 2>&1; do
	tries=$((tries + 1))
	if [ "$tries" -ge 30 ]; then
		cat /work/ping.txt >&2
		fail "the lighthouse got no answer from $IP"
	fi
	sleep 1
done
printf 'ok 4 apply wrote the certificate and the config, and the lighthouse answers %s\n' "$IP"

tree_before=$(tree_sum "$HOST")
code=0
"$MESHY" bootstrap ghost --mesh "$MESH" >/work/ghost.out 2>/work/ghost.err || code=$?
[ "$code" -eq 2 ] || fail "bootstrap of an unknown node exited $code, want 2"
[ ! -s /work/ghost.out ] || fail "bootstrap of an unknown node printed a script"
grep -qF 'ghost is not a node in mesh.yml' /work/ghost.err ||
	fail "bootstrap of an unknown node did not name the file"
on "root@$HOST" sh -s </work/ghost.out
tree_after=$(tree_sum "$HOST")
[ "$tree_before" = "$tree_after" ] || fail "the refusal changed the host"
printf 'ok 5 bootstrap refused ghost with exit 2 and no script, and the host is unchanged\n'

# A host that already runs the pin asks install for no release, so this case
# needs a mesh file whose pin the host does not run. install then resolves the
# release, and the archive fails its checksum before anything reaches the host.
mkdir -p /work/bad/mesh
sed 's/^  version: v1\.12\.0$/  version: v1.11.0/' "$MESH/mesh.yml" >/work/bad/mesh/mesh.yml
grep -qF 'version: v1.11.0' /work/bad/mesh/mesh.yml || fail "the second mesh file holds the wrong pin"
cp "$RELEASE/nebula-linux-amd64.tar.gz" /work/bad/
awk '{ printf "0000000000000000000000000000000000000000000000000000000000000000  %s\n", $2 }' \
	"$RELEASE/SHASUM256.txt" >/work/bad/SHASUM256.txt
before=$(binary_sum "$HOST")
code=0
"$MESHY" install --node "$NODE" --source /work/bad --mesh /work/bad/mesh >/work/bad.out 2>/work/bad.err || code=$?
[ "$code" -eq 1 ] || fail "install of a wrong checksum exited $code, want 1"
grep -qF 'checksum mismatch for nebula-linux-amd64.tar.gz' /work/bad.err || {
	cat /work/bad.err >&2
	fail "install did not name the checksum mismatch"
}
[ "$(binary_sum "$HOST")" = "$before" ] || fail "install changed the binary before the checksum passed"
printf 'ok 6 install refused a release with a wrong checksum and left the binary alone\n'

calls_before=$(calls "$HOST")
before=$(binary_sum "$HOST")
out=$("$MESHY" install --node "$NODE" --source "$RELEASE" --mesh "$MESH")
printf '%s\n' "$out"
printf '%s\n' "$out" | grep -qF "$NODE: $PIN" || fail "install did not report the pin"
case "$out" in
*restarted*) fail "install restarted a host that runs the pin" ;;
esac
[ "$(calls "$HOST")" = "$calls_before" ] || fail "install recorded a call on a host that runs the pin"
[ "$(binary_sum "$HOST")" = "$before" ] || fail "install rewrote the binary on a host that runs the pin"
printf 'ok 7 install on a host that runs %s restarted nothing\n' "$PIN"

printf 'e2e: 7 assertions passed\n'
