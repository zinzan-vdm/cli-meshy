---
tags:
  - bootstrap
  - systemd
---

# The bootstrap script owns the host steps

`meshy bootstrap` prints a shell script. The command never runs that script and never opens a connection. A maintainer reads the script and runs it with `sudo`.

The script prepares the node user, the sudoers rule, `/etc/nebula/meshy-<mesh>/`, and `nebula@meshy-<mesh>.service`. The last two names come from `internal/node`, which `plan` and `apply` also read, so the three commands cannot drift.

## Why a script and not a command

Every step needs root on the node. `apply` runs as the node user over SSH, so it cannot prepare a host that it cannot yet trust. A script shows the maintainer the four steps before any of them run. The same script repairs a node that lost one of them, because each step checks first.

## Why the sudoers rule names the commands

`apply` runs `sudo systemctl reload nebula@meshy-<mesh>.service`. The rule allows `reload` and `restart` of that unit and nothing else. A rule for `systemctl` alone would let the node user start any unit file that the account can write. So the rule lists two commands with the full unit name.

`install` needs a third command, because `/usr/local/bin` belongs to root. That command names its source, its destination, its owner and its mode:

```
<install> -o root -g root -m 0755 /etc/nebula/meshy-<mesh>/nebula.incoming /usr/local/bin/nebula
```

sudo matches a command by its path and by its arguments. The node user can place that one file at that one path, and no other file anywhere. The binary reaches `<dir>/nebula.incoming` through the write path, as the node user, which needs no privilege. The rule holds three commands and no wildcard.

A node that ran an older script holds a rule with two commands. `install` fails there until the maintainer runs `bootstrap` again, and the script replaces the rule when the text differs.

The script writes the path of `systemctl` that the host resolves. sudo matches a command by its full path, and that path differs between distributions.

## Why the script writes the unit

A node must hold the unit before `apply` can reload it. The unit holds fixed content, so the script writes it once and compares it on every later run. A changed unit is installed, then `daemon-reload` runs, then the unit restarts only if it was active.

## Consequences

A host that never ran `bootstrap` holds no unit, so `apply` fails and names this command.

The script comes from `mesh.yml`. A name that the file does not hold gets no script and exit code 2.
