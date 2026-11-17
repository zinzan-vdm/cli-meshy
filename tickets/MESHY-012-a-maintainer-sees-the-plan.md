---
id: MESHY-012
title: A maintainer sees the plan before it runs
type: task
status: todo
priority: high
parent: MESHY-008
depends_on:
  - MESHY-009
  - MESHY-010
  - MESHY-011
tags: [plan, cli, ci]
---

## Overview

`meshy plan` prints the difference between the mesh file and the nodes, in text for a person and as JSON for a workflow. The exit code is the signal: zero for no change, one for a change.

## Context / Background

A pull request check must fail when the mesh would change on merge, or the review is blind. The same command runs in CI and on a laptop, so the text and the JSON come from one plan.

Part of [MESHY-008](MESHY-008-see-what-a-merge-will-change.md): a maintainer can see what a merge will change.

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

- [MESHY-008](MESHY-008-see-what-a-merge-will-change.md): the parent epic.
- [MESHY-023](MESHY-023-a-pull-request-plans-the-mesh.md): the workflow that consumes the JSON.

## Acceptance Criteria

- [ ] A mesh with no differences exits 0.
- [ ] Any difference exits 1.
- [ ] Each difference names the node and the kind.
- [ ] `--json` prints the shape above and nothing else on stdout.
- [ ] A user node is marked `unread` and reports only its certificate.
- [ ] The output is sorted, so two runs print the same text.
- [ ] A node that fails the read appears with the failure, and the plan still exits 1.
