#!/bin/sh
# The node starts an sshd and nothing else. meshy writes the first config over
# ssh and starts the unit, so the container needs no config of its own.
set -e

key="${MESHY_AUTHORIZED_KEY:-/run/meshy/authorized_key}"
if [ ! -f "$key" ]; then
  echo "entrypoint: no authorized key at $key" >&2
  exit 1
fi

install -d -o meshy -g meshy -m 0700 /home/meshy/.ssh
install -o meshy -g meshy -m 0600 "$key" /home/meshy/.ssh/authorized_keys

mkdir -p /run/sshd /etc/nebula /etc/systemd/system /var/log
# bootstrap creates this directory on a real node, and meshy writes the config
# into it. The fixture stands where bootstrap stands.
install -d -o meshy -g meshy -m 0700 /etc/nebula/meshy-rot

# The unit that meshy checks before it writes. The shim answers for it.
printf '[Unit]\nDescription=nebula meshy-rot\n' > /etc/systemd/system/nebula@meshy-rot.service

# The host key of this container, so known_hosts can name it.
ssh-keygen -A

exec /usr/sbin/sshd -D -e
