---
# MESHY-042
title: Apply reports one line per node
status: todo
type: task
priority: high
tags:
    - apply
    - cli
created_at: 2026-11-18T00:40:00Z
updated_at: 2026-11-18T00:40:00Z
parent: MESHY-013
blocked_by:
    - MESHY-014
    - MESHY-015
    - MESHY-016
    - MESHY-041
---

## Overview

`meshy apply` runs the steps of the epic across every node, keeps going when one node fails, and prints one line for each node. The exit code says what happened.

## Context / Background

The other tasks in the epic each change one node. This task owns the command: the order, the isolation between nodes, and the exit contract. A reviewer sees the whole run in one summary.

The exit contract follows [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md). 0 means the mesh matches. 1 means a difference a person must see. 2 means the run never started.

## Specification

```
meshy apply [--mesh <dir>] [--node <name>] [--dry-run] [--json]
```

| Step | Behaviour |
|---|---|
| 1 | load `mesh.yml` and validate it |
| 2 | open the vault, or stop with the exit code for a run that never started |
| 3 | read every node, as `plan` does |
| 4 | for each node: issue or renew a certificate, write what differs, reload once |
| 5 | push a blocklist when a node left the mesh |
| 6 | print one line per node |

- A node that fails does not stop the others. The exit code is 1 and the line names the failure.
- A mesh that fails validation writes nothing and exits 2.
- A second run against an unchanged mesh writes nothing and exits 0.
- `--node` limits the run to one node. An unknown name exits 2.
- `--dry-run` is `plan`. It writes nothing and prints the plan that `meshy plan` prints.

## Technical Notes

- Every step is idempotent, so the same input produces no write.
- The summary goes to stdout. Each line starts with the node name, then what changed.
- `--json` prints the report of [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md), extended with the action taken for each change.
- Apply never installs a binary and never changes the pinned version. That is [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md).

## Reference Material

- [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): the epic this completes.
- [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md): the read side and the exit codes.
- [ADR 0004](../docs/adr/0004-a-pull-request-is-the-gate.md): the review is where a change is approved.

## Acceptance Criteria

- [ ] One failing node leaves the others applied, and the exit code is 1.
- [ ] The summary names the failed node and the reason.
- [ ] A mesh that fails validation writes nothing and exits 2.
- [ ] A second run writes nothing and exits 0.
- [ ] `--node` reaches one node and refuses an unknown name with exit 2.
- [ ] `--dry-run` writes nothing and prints the same report as `meshy plan`.
- [ ] Apply changes no binary.
