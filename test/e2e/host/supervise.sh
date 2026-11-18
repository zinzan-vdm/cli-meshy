#!/bin/sh
# Stands in for nebula@meshy-<mesh>.service. A container has no systemd, so this
# loop starts the binary from the same path and the same config that the unit
# names, and it sends SIGHUP when the config changes. The unit file itself is
# never started. See docs/patterns/0022.
set -u

DIR=/etc/nebula/meshy-hostlab
BINARY=/usr/local/bin/nebula
CONFIG="$DIR/config.yml"

# apply writes the key last, so a key on disk means that all five files are
# there. The loop starts nothing before that.
ready() {
	[ -f "$CONFIG" ] || return 1
	for key in "$DIR"/*.key; do
		[ -f "$key" ] && return 0
	done
	return 1
}

while :; do
	until ready; do
		sleep 1
	done

	"$BINARY" -config "$CONFIG" &
	pid=$!
	stamp=$(stat -c %Y "$CONFIG")

	while kill -0 "$pid" 2>/dev/null; do
		sleep 1
		now=$(stat -c %Y "$CONFIG" 2>/dev/null || printf '%s' "$stamp")
		if [ "$now" != "$stamp" ]; then
			stamp=$now
			kill -HUP "$pid" 2>/dev/null || true
		fi
	done

	wait "$pid" 2>/dev/null || true
	sleep 1
done
