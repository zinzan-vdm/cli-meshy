---
# MESHY-045
title: The unit reloads when it is told to
status: completed
type: task
priority: high
tags:
    - apply
    - systemd
    - defect
created_at: 2026-11-18T01:20:00Z
updated_at: 2026-11-18T01:45:00Z
parent: MESHY-013
---

## Overview

`apply` refreshes the unit after a write. It reloads a unit that runs, and it starts a unit that does not. The unit the script writes answers a reload.

## Context / Background

Two defects meet on the same line of the run, and neither is visible with a fake that answers every command.

The first is in the unit file. [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md) writes the unit with `ExecStart` and no `ExecReload`. A unit that runs and holds no `ExecReload` refuses a reload:

```
Failed to reload nebula@meshy-homelab.service: Job type reload is not applicable for unit nebula@meshy-homelab.service.
```

So `apply` failed at the last step on every node, warm or cold.

The second is in the order. The script enables the unit and leaves it stopped, because its restart is guarded by `systemctl is-active`. `meshy install` then restarts the node that changed, and at that moment no `config.yml` exists. `ExecStart` names that file, so Nebula exits and the unit stays down. A reload on a stopped unit fails too:

```
nebula@meshy-homelab.service is not active, cannot reload.
```

## Specification

| State of the unit | `apply` runs | Because |
|---|---|---|
| active | `sudo -n systemctl reload <unit>.service` | the running process takes a `SIGHUP` and keeps its tunnels |
| not active | `sudo -n systemctl restart <unit>.service` | a reload cannot start a unit, and a restart can |
| no write happened | nothing | a run that changes nothing touches nothing |

The unit gains one line:

```
ExecReload=/bin/kill -HUP $MAINPID
```

Both commands are already allowed by the sudoers rule that [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md) writes, so no privilege changes.

## Technical Notes

- `systemctl is-active` exits 3 for a unit that does not run, so the state is read from its output and not from its exit code.
- `restart` starts a unit that is stopped, and it clears a failed state. A `start` would do the same, but the sudoers rule names `reload` and `restart`, so `restart` is the command that needs no new rule.
- The read back after the refresh still requires an active unit, so a unit that refuses to start is reported.

## Reference Material

- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write path this completes.
- [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md): the unit file and the sudoers rule.
- [0010](../docs/patterns/0010-the-unit-reloads-once-after-the-last-write.md): one refresh, after the last write.
- [0011](../docs/patterns/0011-a-stopped-unit-is-started-not-reloaded.md): the rule this ticket adds.

## Acceptance Criteria

- [x] The unit that the script writes names `ExecReload=/bin/kill -HUP $MAINPID`.
- [x] A unit that runs is reloaded and not restarted.
- [x] A unit that does not run is restarted and not reloaded.
- [x] A run that writes nothing runs neither command.
- [x] The read back still requires an active unit.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass.

## Summary of Changes

The unit answers a reload, and `apply` starts a unit that does not run.

### Files

- `internal/bootstrap/bootstrap.go`: the unit gains `ExecReload=/bin/kill -HUP $MAINPID`.
- `internal/apply/apply.go`: `refresh` reads the unit state and runs one command. `Result.Action` replaces the old `Reloaded` flag, so a caller can tell a reload from a start.
- `cmd/meshy/apply.go`: the report says `reloaded` or `started`.
- `internal/apply/apply_test.go`, `cmd/meshy/apply_test.go`, `internal/bootstrap/bootstrap_test.go`: the new rules, plus a fake that can hold a unit down.
- `docs/patterns/0011-a-stopped-unit-is-started-not-reloaded.md`, and 0010 points at it.

### Decisions

- The command is `restart` and not `start`, because the sudoers rule that MESHY-027 writes names `reload` and `restart`. A new rule would be a new privilege for no gain.
- The state is read from the output of `systemctl is-active`, not from its exit code. A unit that does not run exits 3, and a failure to run the command is a different thing.

### Deviations

None.

### Measurements

- `gofmt -l .` silent, `go build ./...` and `go vet ./...` pass.
- 190 test results across 13 packages, 2 more than before.
- Both commands were proved on a real systemd before the code changed: a reload on an active unit without `ExecReload` exits 3 with `Job type reload is not applicable`, a reload on a stopped unit exits 1 with `is not active, cannot reload`, and a restart on a stopped unit exits 0 and leaves it active.
