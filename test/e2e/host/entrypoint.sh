#!/bin/sh
# The entrypoint of the lighthouse and of the empty host. It starts sshd and then
# runs the process that stands in for the systemd unit. A container has no
# systemd, so nothing here starts a unit.
set -eu

mkdir -p /run/sshd /root/.ssh /etc/systemd/system /var/log
chmod 0700 /run/sshd /root/.ssh

# The runner holds the only private key. The compose file mounts the public key
# read only, and this copy gives the owner of the file to root alone.
install -m 0600 /run/keys/authorized_keys /root/.ssh/authorized_keys

# The host keys are new in every container, and the runner scans them before it
# connects.
ssh-keygen -q -A

/usr/sbin/sshd

exec /usr/local/bin/supervise
