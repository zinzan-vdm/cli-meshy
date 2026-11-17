---
id: MESHY-027
title: Bootstrap a new server node
type: task
status: todo
priority: normal
parent: MESHY-013
tags: [ssh, bootstrap]
---

## Overview

`meshy bootstrap <node>` prints an idempotent script that prepares a server node: the user, the sudoers rule, the config directory, and the systemd unit. The maintainer runs it once. The command never runs it.

## Context / Background

`apply` writes configuration and reloads a unit that must already exist. The gap between a fresh server and a manageable node is four small steps, and doing them by hand produces a node that half works.

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

- [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-014](MESHY-014-write-config-and-reload.md): the write that needs the unit.
- [MESHY-019](MESHY-019-ship-a-platform-installer.md): the shared unit template.

## Acceptance Criteria

- [ ] The output is a shell script and nothing else.
- [ ] The script is idempotent, so a second run changes nothing.
- [ ] The unit name matches the name `apply` reloads.
- [ ] The sudoers rule allows reload and restart of that unit only.
- [ ] A user node is refused with a clear message.
- [ ] The command makes no network connection.
