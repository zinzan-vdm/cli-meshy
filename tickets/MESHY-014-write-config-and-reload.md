---
id: MESHY-014
title: Write config and reload a server node
type: task
status: todo
priority: high
parent: MESHY-013
depends_on:
  - MESHY-011
  - MESHY-012
tags: [apply, ssh, nebula]
---

## Overview

`meshy` writes the rendered config and the certificate to a server node, then reloads the unit. It writes only when the content differs, so a run that changes nothing touches nothing.

## Context / Background

Nebula reloads its firewall and its certificates on `SIGHUP` without dropping tunnels, so a reload is cheap and safe. The write is the risky part, and it must be atomic: a half written config stops the service on the next restart.

## Specification

Files on the node, under `/etc/nebula/meshy-<mesh>/`:

| File | Mode | Owner |
|---|---|---|
| `config.yml` | 0600 | the node user |
| `ca.crt` | 0644 | the node user |
| `<node>.crt` | 0644 | the node user |
| `<node>.key` | 0600 | the node user |

Write order:

1. write each changed file to `<name>.tmp` in the same directory
2. rename it into place
3. run `systemctl reload nebula@meshy-<mesh>` once, after all writes
4. read the unit state and the config hash back

The unit is `nebula@meshy-<mesh>.service`, installed by `meshy bootstrap`. A missing unit is an error that names the bootstrap command.

## Technical Notes

- Uploads use the SSH session, so no `scp` binary is needed on the control host.
- The reload uses `sudo systemctl reload` through the node user's sudoers rule.
- A failed rename leaves the old file in place, because the rename is the last step.
- The read back compares the hash and the unit state, so a silent failure is visible.

## Reference Material

- [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-011](MESHY-011-read-live-state-over-ssh.md): the read side.
- [MESHY-019](MESHY-019-ship-a-platform-installer.md): the unit file for user nodes.

## Acceptance Criteria

- [ ] A config that matches the node is not written.
- [ ] A changed config is written through a temporary file and a rename.
- [ ] The unit reloads once, after every write.
- [ ] The node reads back the new hash and an active unit.
- [ ] A missing unit exits 1 and names `meshy bootstrap`.
- [ ] File modes on the node are 0600 for the config and the key.
- [ ] A write that fails leaves the previous config in place.
