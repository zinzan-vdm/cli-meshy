---
id: MESHY-032
title: A merge applies the mesh
type: task
status: todo
priority: high
parent: MESHY-022
depends_on:
  - MESHY-014
  - MESHY-023
tags: [ci, automation]
---

## Overview

A merge to `main` runs `meshy apply`, so the nodes change when the reviewed change lands. A failure leaves the vault closed on the runner.

## Context / Background

The pull request showed the plan. The merge is the approval, so the same workflow applies the plan without a second gate. The apply job holds the shares, which is why it runs only on the trusted branch.

Part of [MESHY-022](MESHY-022-ci-runs-the-mesh.md): cI runs the mesh.

## Specification

```yaml
on:
  push:
    branches: [main]
  schedule:
    - cron: "0 3 * * 1"
  workflow_dispatch:
```

| Step | Command | Note |
|---|---|---|
| unlock | `meshy unlock` | reads `MESHY_SHARE_1..3` from the environment |
| apply | `meshy apply` | writes only where the content differs |
| lock | `meshy lock` | runs with `if: always()` |

The scheduled run applies only when the plan reports a renewal or drift.

## Technical Notes

- Secrets: `MESHY_SHARE_1`, `MESHY_SHARE_2`, `MESHY_SHARE_3`, `MESHY_SSH_KEY`.
- The lock step runs even when the apply step fails, so a failed run does not leave the vault open.
- The job prints the plan it applied to `$GITHUB_STEP_SUMMARY`.
- Every action is pinned by commit SHA.

## Reference Material

- [MESHY-022](MESHY-022-ci-runs-the-mesh.md): the parent epic.
- [MESHY-014](MESHY-014-apply-the-config-to-a-node.md): the apply on one node.
- [MESHY-005](MESHY-005-unseal-the-vault-for-a-run.md): the unlock and lock commands.

## Acceptance Criteria

- [ ] A merge to `main` runs `apply`.
- [ ] The lock step runs even when the apply step fails.
- [ ] The weekly run applies when the plan reports a renewal.
- [ ] A failed apply leaves no plaintext key on the runner.
- [ ] The job prints the plan it applied.
- [ ] No step prints a share or a key.
