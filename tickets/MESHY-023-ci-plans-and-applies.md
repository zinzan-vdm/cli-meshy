---
id: MESHY-023
title: Plan on a pull request and apply on merge
type: task
status: todo
priority: high
parent: MESHY-022
depends_on:
  - MESHY-005
  - MESHY-012
tags: [ci, automation]
---

## Overview

The workflow in a mesh repository runs the plan on every pull request and the apply on every merge to `main`. It also runs on a schedule, which is the safety net for renewal and drift.

## Context / Background

The reviewer approves the diff, not a button. That works only when the pull request shows what the diff will do to the nodes, which is the plan.

## Specification

```yaml
on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: "0 3 * * 1"
  workflow_dispatch:
```

| Job | Condition | Steps |
|---|---|---|
| `validate` | always | checkout, `meshy validate` |
| `plan` | pull request, same repository | checkout, install, `meshy plan --json` |
| `apply` | push to `main`, schedule, dispatch | checkout, install, unlock, apply, lock |

`apply` runs `meshy lock` with `if: always()`, so a failure does not leave the vault open on the runner.

## Technical Notes

- The workflow file lives at `.github/workflows/meshy.yml` in the mesh repository.
- Secrets: `MESHY_SHARE_1`, `MESHY_SHARE_2`, `MESHY_SHARE_3`, `MESHY_SSH_KEY`.
- The plan job writes its JSON to `$GITHUB_STEP_SUMMARY`.
- A scheduled run applies only when the plan reports a renewal or drift.

## Reference Material

- [MESHY-022](MESHY-022-ci-runs-the-mesh.md): the parent epic.
- [MESHY-005](MESHY-005-open-and-close-the-vault.md): the unlock and lock commands.
- [MESHY-012](MESHY-012-report-the-plan.md): the JSON the plan prints.

## Acceptance Criteria

- [ ] A pull request runs `validate` and `plan` and fails on a difference.
- [ ] A pull request from a fork runs `validate` only.
- [ ] A merge to `main` runs `apply`.
- [ ] The lock step runs even when the apply step fails.
- [ ] The weekly run applies when the plan reports a renewal.
- [ ] No step prints a share or a key.
- [ ] The workflow pins every action by commit SHA.
