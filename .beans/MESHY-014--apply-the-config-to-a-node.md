---
# MESHY-014
title: Apply the config to a node
status: completed
type: task
priority: high
tags:
    - apply
    - ssh
    - nebula
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-18T00:01:39Z
parent: MESHY-013
blocked_by:
    - MESHY-011
    - MESHY-012
---
## Overview

`meshy` writes the rendered config and the certificate to a server node, then reloads the unit. It writes only when the content differs, so a run that changes nothing touches nothing.

## Context / Background

Nebula reloads its firewall and its certificates on `SIGHUP` without dropping tunnels, so a reload is cheap and safe. The write is the risky part, and it must be atomic: a half written config stops the service on the next restart.

Part of [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): a merge changes the mesh.

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

- [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-011](MESHY-011--a-maintainer-sees-what-a-node-runs.md): the read side.
- [MESHY-019](MESHY-019--ship-a-platform-installer.md): the unit file for user nodes.

## Acceptance Criteria

- [x] A config that matches the node is not written.
- [x] A changed config is written through a temporary file and a rename.
- [x] The unit reloads once, after every write.
- [x] The node reads back the new hash and an active unit.
- [x] A missing unit exits 1 and names `meshy bootstrap`.
- [x] File modes on the node are 0600 for the config and the key.
- [x] A write that fails leaves the previous config in place.

## Summary of Changes

Files:

- `internal/apply/apply.go`: the `Session` interface, `Files`, `Result`, and `Run`. `Session` adds `Write` and `Read` to `node.Executor`, so the read-only contract stays as it is. `Run` checks the unit. It then writes the four files in the order of the specification, reloads once, and reads the hash and the unit state back. The atomic step is one function, `replace`.
- `internal/apply/apply_test.go`: 11 cases with a fake session. The fake lives in this test file and answers the commands that `Run` builds, so it cannot drift from them.
- `internal/sshx/write.go`: `Write` and `Read`. The bytes of a write travel on the session's stdin. A path that the node does not hold reports `node.ErrNoFile`, which a separate exit code keeps apart from a failed read.
- `internal/sshx/write_test.go`: 6 cases over a local SSH server. They assert the stdin bytes, the mode, and that the command holds no `scp`, no `sftp`, and no heredoc.
- `internal/sshx/client.go`: `Run` now calls an internal `run` that takes an optional stdin. The timeout and the exit status logic stay in one place.
- `internal/node/read.go`: `ErrNoFile`.
- `cmd/meshy/apply.go`: `cmdApply`, `applyFiles`, `applyNode`, `reportOne`, and the `dialSession` seam. One comment marks the place where MESHY-042 adds the loop.
- `cmd/meshy/apply_test.go`: 12 cases. They cover the exit codes, the two refusal messages, the vault material, and the report.
- `cmd/meshy/main.go`: the `apply` case and its usage line.
- `docs/patterns/0009-a-write-to-a-node-replaces-the-file-by-rename.md`, `docs/patterns/0010-the-unit-reloads-once-after-the-last-write.md`, `docs/patterns/TAGS`: the two rules that a reader will question, with the tags `apply` and `ssh`.
- `README.md`: the status line, a section on `apply`, and the command table.

Decisions:

- The exit codes follow the brief. A missing certificate and a closed vault return 2, because the run never started. A dial failure and a failed run return 1, because the node failed.
- A user node returns 1 with the message of `meshy bootstrap`, because a user node gets a bundle rather than a unit.
- The reload names `<unit>.service`. The sudoers rule that MESHY-027 writes matches the whole argument, so a reload without the suffix would be refused.
- The rename goes through `Session.Run` with `mv -f`. That keeps the wider interface at two methods and puts the atomic step in one function.
- The read back runs on every successful run, not only after a write. A node whose unit is down is reported, not passed.
- The command test replaces `dialSession` with a fake session, so the exit code and the message are proved with no node.

Deviations:

- A file whose content matches is not written, and its mode is not repaired. The ticket compares content, and the session interface does not report a mode. A file that a person chmod'ed by hand keeps the wrong mode until its content changes.
- `ca.crt` holds the certificate of `ca.active` only. The trust bundle of `ca.trust` is [MESHY-025](MESHY-025--every-node-trusts-the-next-authority.md).
- An open conflict: on a host that `bootstrap` prepared and that no command started, the unit is inactive, and `systemctl reload` fails on an inactive unit. `apply` reports that failure with exit 1 and the unit name. This ticket does not decide who starts the unit. [MESHY-043](MESHY-043--a-host-becomes-a-node.md) gives the restart to `install`.
- `README.md` also marks `ca init`, `issue`, `plan`, `unlock`, and `lock` as working. They already worked, and the status line had to change for `apply`.
