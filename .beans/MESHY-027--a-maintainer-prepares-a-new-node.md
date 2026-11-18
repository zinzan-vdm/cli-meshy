---
# MESHY-027
title: A maintainer prepares a new server node
status: completed
type: task
priority: normal
tags:
    - ssh
    - bootstrap
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T02:20:00Z
parent: MESHY-043
---
## Overview

`meshy bootstrap <node>` prints an idempotent script that prepares a server node: the user, the sudoers rule, the config directory, and the systemd unit. The maintainer runs it once. The command never runs it.

## Context / Background

`apply` writes configuration and reloads a unit that must already exist. The gap between a fresh server and a manageable node is four small steps, and doing them by hand produces a node that half works.

Part of [MESHY-043](MESHY-043--a-host-becomes-a-node.md): a merge changes the mesh.

## Specification

```
meshy bootstrap <node> [--mesh <dir>]
```

Output: a shell script on stdout, and nothing else. The script:

1. creates the user from `ssh.user` when it is absent
2. writes `/etc/sudoers.d/meshy-<mesh>` with one rule: `systemctl reload|restart nebula@meshy-<mesh>`
3. creates `/etc/nebula/meshy-<mesh>/` with mode 0700
4. writes the unit `nebula@meshy-<mesh>.service` and runs `systemctl daemon-reload`

Every step checks first, so a second run changes nothing.

| Condition | Behaviour |
|---|---|
| node is `kind: user` | exit 1, `horo-laptop is a user node, run meshy bundle` |
| node has no `ssh` | exit 1, and the validator already rejects this |

## Technical Notes

- The script is generated from the same values `apply` uses, so the unit name and the config path cannot drift.
- The command prints and does not connect. That keeps a destructive step in the maintainer's hands.
- The unit file is the same template the Linux installer ships.

## Reference Material

- [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write that needs the unit.
- [MESHY-019](MESHY-019--ship-a-platform-installer.md): the shared unit template.

## Acceptance Criteria

- [x] The output is a shell script and nothing else.
- [x] The script is idempotent, so a second run changes nothing.
- [x] The unit name matches the name `apply` reloads.
- [x] The sudoers rule allows reload and restart of that unit only.
- [x] A user node is refused with a clear message.
- [x] The command makes no network connection.

## Summary of Changes

Files:

- `internal/bootstrap/bootstrap.go`: `Script(mesh, node)` returns the shell script as a string. It uses a `text/template` and the two names from `internal/node`, `Unit` and `Dir`, so the names match `plan` and `apply`. It reads no file and opens no connection.
- `internal/bootstrap/bootstrap_test.go`: 9 cases. They cover the four steps, the unit name, and the two commands in the sudoers rule. They also cover the guard before every change and a `sh -n` syntax check. Two more cases cover quoting, and two cover a refusal.
- `cmd/meshy/bootstrap.go`: `cmdBootstrap`. It loads the mesh, validates it, finds the node, and prints the script.
- `cmd/meshy/bootstrap_test.go`: 8 cases. They cover a clean print and the refusal of a user node with its exact message. They also cover an unknown node, a mesh file that does not load, and a mesh file with problems. The last two cover a missing node name, and the unit and directory names.
- `cmd/meshy/main.go`: the `bootstrap` case and its usage line.
- `docs/adr/0009-the-bootstrap-script-owns-the-host.md`, `docs/adr/TAGS`: the reason for a script, the reason the sudoers rule names two commands, and the reason the script writes the unit.
- `README.md`: a section that says how to run the script and what it does to a host.

Decisions:

- The exit codes follow `cmd/meshy/plan.go`. A missing node name, a mesh file that does not load, a mesh file with problems, and an unknown node return 2. A user node and a node without `ssh.user` return 1.
- The sudoers rule holds the path of `systemctl` that the host resolves, taken at run time. sudo matches a command by its full path, and that path differs between distributions.
- The rule is written to a temporary file, checked with `visudo -cf`, then moved into place. A rule that fails the check stops the script.
- The script resolves flags after the node name by parsing the remainder of the line again. The `flag` package stops at the first positional argument, and the usage writes the node first.

Deviations:

- The script also runs `systemctl enable` on the unit, inside the branch that installs a changed file. The specification stops at `daemon-reload`. Without `enable`, a node that `meshy install` starts comes back down after a reboot, which is the half working node that this ticket exists to prevent.
- The script gives `/etc/nebula/meshy-<mesh>/` to the node user. The specification sets the mode and says nothing about the owner. `apply` writes `config.yml` into that directory as the node user, and mode 0700 means nobody else can.
- The unit template lives in `internal/bootstrap`. The ticket says it is the same template that the Linux installer ships, but `internal/scaffold/installers/` does not exist yet, because MESHY-018 has not started. When MESHY-018 lands, the two must share one template.

## Note

MESHY-043 found a defect in this script. The node user was created with `/usr/sbin/nologin`, and sshd refuses a remote command for that shell. So `install` and `apply` could not reach a host that the script had just prepared. The script now sets `/bin/sh` and repairs a host that an older copy prepared. ADR 0009 records the decision, and `internal/bootstrap/bootstrap_test.go` holds both lines.
