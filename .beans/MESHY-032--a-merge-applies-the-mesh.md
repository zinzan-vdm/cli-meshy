---
# MESHY-032
title: A merge applies the mesh
status: completed
type: task
priority: high
tags:
    - ci
    - automation
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-18T00:00:00Z
parent: MESHY-022
blocked_by:
    - MESHY-014
    - MESHY-023
---
## Overview

A merge to `main` runs `meshy apply`, so the nodes change when the reviewed change lands. A failure leaves the vault closed on the runner.

## Context / Background

The pull request showed the plan. The merge is the approval, so the same workflow applies the plan without a second gate. The apply job holds the shares, which is why it runs only on the trusted branch.

Part of [MESHY-022](MESHY-022--ci-runs-the-mesh.md): cI runs the mesh.

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

- [MESHY-022](MESHY-022--ci-runs-the-mesh.md): the parent epic.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the apply on one node.
- [MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md): the unlock and lock commands.

## Acceptance Criteria

- [x] A merge to `main` runs `apply`.
- [x] The lock step runs even when the apply step fails.
- [x] The weekly run applies when the plan reports a renewal.
- [x] A failed apply leaves no plaintext key on the runner.
- [x] The job prints the plan it applied.
- [x] No step prints a share or a key.

## Summary of Changes

| File | Change |
|---|---|
| `internal/scaffold/scaffold.go` | The `workflow` template gains the `push` trigger for `main`, the `apply` job, the share step, the `nebula-cert` step, the plan gate, the apply step, the `always()` lock, and the `always()` cleanup. |
| `internal/scaffold/workflow_test.go` | The trigger and job tests move to four triggers and three jobs. Seven new tests cover the merge event, the shares, the plan gate, the lock and the cleanup order, the signing tool, the step summary, and the empty vault in the plan job. The test that forbade an apply job is removed. |
| `docs/conf/0002-the-mesh-workflow.md` | Adds the three shares, the merge and apply paths, the plan gate, the signing tool, and the step summary. States what is unproven on a runner. |
| `docs/patterns/0019-the-vault-opens-only-for-a-change.md` | The rule that the apply job plans with a closed vault and unseals only for a change or a merge. |
| `README.md` | States that a merge applies and that a weekly run applies on drift or renewal. |
| `.beans/MESHY-032--a-merge-applies-the-mesh.md` | This ticket. |

## Deviations

1. **The apply job plans before it unseals.** The vault stays closed while the job reads the nodes. A quiet week never puts the authority key on the runner. A plan without the vault cannot compare a node certificate with the vault copy, so that one difference waits for the apply. The merge path still applies, and the apply writes every file that differs.
2. **The gate reads the exit code, not the count.** The first cut read `.changes` with `jq`. That added a tool the workflow never needed, and a runner image without it failed the gate. `meshy plan` already exits 1 when a difference exists, 0 on a match, and 2 when it produced no plan, so the shell alone decides. A test asserts that the workflow names no `jq`.
3. **The signing tool comes from a hard-coded platform.** The download names `nebula-linux-amd64.tar.gz`, because `ubuntu-latest` is `linux-amd64`. The step is unproven on a runner, and `docs/conf/0002` says so. The URL and the digest rule come from `internal/release` and pattern 0012.
4. **A new pattern is 0019.** The rule that the vault opens only for a change passes the three tests in the pattern skill. No existing pattern states it. The number is 0019 because MESHY-018 took 0017 and 0018 on a parallel branch.
5. **The plan job also runs on the weekly and manual run.** It reports on the run page even when the apply job applies nothing.
