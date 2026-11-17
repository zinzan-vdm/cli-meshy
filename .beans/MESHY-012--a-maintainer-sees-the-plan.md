---
# MESHY-012
title: A maintainer sees the plan before it runs
status: completed
type: task
priority: high
tags:
    - plan
    - cli
    - ci
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T22:05:02Z
parent: MESHY-008
blocked_by:
    - MESHY-009
    - MESHY-010
    - MESHY-011
---

## Overview

`meshy plan` prints the difference between the mesh file and the nodes, in text for a person and as JSON for a workflow. The exit code is the signal: zero for no change, one for a change.

## Context / Background

A pull request check must fail when the mesh would change on merge, or the review is blind. The same command runs in CI and on a laptop, so the text and the JSON come from one plan.

Part of [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): a maintainer can see what a merge will change.

## Specification

```
meshy plan [--json] [--mesh <dir>]
```

Text output, one block per node:

```
web-1   certificate   expires in 12d, renews
web-1   config        differs
core-1  binary        runs v1.11.0, pinned v1.12.0
core-1  ok
```

JSON output:

```json
{
  "mesh": "homelab",
  "nodes": [
    {
      "name": "web-1",
      "changes": [
        {"kind": "certificate", "detail": "expires in 12d"}
      ]
    }
  ],
  "changes": 1
}
```

| Kind | Meaning |
|---|---|
| `certificate` | missing, expiring inside the window, or different from the vault |
| `config` | the rendered config hash differs from the node |
| `binary` | the running version differs from the pin |
| `service` | the unit is not active |
| `interface` | the device is missing or holds another address |

User nodes are never read. The plan reports their certificate from the vault and marks the rest `unread`.

## Technical Notes

- `internal/plan` builds the report. The command only formats it.
- The JSON shape is stable and versioned with the schema, because a workflow parses it.
- The text output is sorted by node name, then kind.

## Reference Material

- [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): the parent epic.
- [MESHY-023](MESHY-023--a-pull-request-plans-the-mesh.md): the workflow that consumes the JSON.

## Acceptance Criteria

- [x] A mesh with no differences exits 0.
- [x] Any difference exits 1.
- [x] Each difference names the node and the kind.
- [x] `--json` prints the shape above and nothing else on stdout.
- [x] A user node is marked `unread` and reports only its certificate.
- [x] The output is sorted, so two runs print the same text.
- [x] A node that fails the read appears with the failure, and the plan still exits 1.

## Summary of Changes

- `internal/plan/plan.go`: `Report`, `Node`, `Change`, `Kind`, `Reading`, `Input`, `Compute`, and `Report.Clean`. `Compute` takes the mesh, the readings, the certificate records, and the clock, and returns the report. It opens no connection and reads no clock of its own.
- `internal/plan/plan_test.go`: 13 tests and 14 cases, with no SSH and no node. Each kind of difference has a case, and the renewal window has a boundary case.
- `cmd/meshy/plan.go`: `cmdPlan`, the text form, and the JSON form.
- `cmd/meshy/plan_test.go`: 5 tests for the two forms and for the node name in a read error.
- `cmd/meshy/plan_ssh_test.go`: 5 tests against a local SSH server: exit 0, exit 1, JSON, an unset `MESHY_KNOWN_HOSTS`, and an invalid mesh.
- `cmd/meshy/main.go`: the dispatch case and one usage line.

Exit codes: 0 when every node matches, 1 when the plan holds a difference, and 2 when no plan could be produced. The second code covers a bad flag, a mesh file that fails validation, an unset `MESHY_KNOWN_HOSTS`, and a node that could not be read.

Three changes from the specification.

The vault is not in this build. MESHY-005, MESHY-006, and MESHY-007 are still open, so `internal/plan` takes the certificate records as a field of `Input` and the command passes none. A user node is still marked `unread`, and it reports a certificate fact when a record is present. The command reads the vault in the ticket that adds it.

A node that fails the read is reported in an `error` field on the node, not as a change, so the five kinds stay the schema. The plan still exits 1, because a plan that could not read a node cannot claim that the node matches. The same field carries a node in the file with no reading, and a read that names no node in the file.

The text form prints `ok` only for a node with no difference. The example in the Specification shows `core-1  ok` below a `core-1  binary` line. A node with a difference is not `ok`.

One implementation note. A certificate can carry the mesh prefix in its address, such as `10.42.0.2/24`, where the file writes `10.42.0.2`. The plan drops the prefix before it compares, so a matching node is not reported as a wrong address.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | every node matches the mesh file |
| 1 | there is at least one difference |
| 2 | no plan could be produced: a bad flag, a mesh file that fails validation, an unset `MESHY_KNOWN_HOSTS`, or a node that could not be read |

A node that cannot be read is not a difference. Code 2 keeps a broken read apart from a real change, so a reviewer never reads a plumbing failure as a plan.
