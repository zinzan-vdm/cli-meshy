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

## Why the node user holds a shell

The node user runs a command over SSH. `apply`, `install`, and `rotate` open a session as that user and run one command. sshd runs that command through the shell of the account and refuses it when the shell is `nologin`.

The first version of this script set `/usr/sbin/nologin`, which looks safer and is not. It denies every command, so a host that the script had just prepared could not be reached at all. The empty-host run in MESHY-043 found the defect, because it runs the script and then opens the session.

The script now sets `/bin/sh`. The session is not interactive, and the account holds no password. The key is the only way in, and the sudoers rule above bounds what the key may run.

## Why the account is not locked

`useradd` creates the node user with a locked password field. sshd refuses every method for a locked account, so the key that the node holds never opens a session. The message in the daemon log is `User meshy not allowed because account is locked`.

The script sets the password field to `*`, which no password hash can match. A password cannot log in, and the account is no longer locked, so sshd accepts the key. The empty-host run in MESHY-043 found this, because it opens a session as that user after it runs the script.
