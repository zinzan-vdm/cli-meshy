---
# MESHY-023
title: A pull request plans the mesh
status: todo
type: task
priority: high
tags:
    - ci
    - automation
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-17T20:54:22Z
parent: MESHY-022
blocked_by:
    - MESHY-005
    - MESHY-012
---
## Overview

The workflow in a mesh repository runs `meshy validate` and `meshy plan` on every pull request. The reviewer sees what the change will do to the nodes before approving it.

## Context / Background

The reviewer approves the diff, not a button. That works only when the pull request shows what the diff will do to the nodes, which is the plan.

Part of [MESHY-022](MESHY-022--ci-runs-the-mesh.md): cI runs the mesh.

## Specification

```yaml
on:
  pull_request:
  schedule:
    - cron: "0 3 * * 1"
  workflow_dispatch:
```

| Job | Condition | Steps |
|---|---|---|
| `validate` | always | checkout, `meshy validate` |
| `plan` | pull request, same repository | checkout, install, `meshy plan --json` |

A pull request from a fork runs `validate` only. It has no secrets, so it cannot reach a node.

## Technical Notes

- The workflow file lives at `.github/workflows/meshy.yml` in the mesh repository.
- The plan job writes its JSON to `$GITHUB_STEP_SUMMARY`.
- The scheduled run reports drift and renewal dates without applying anything.
- Every action is pinned by commit SHA.

## Reference Material

- [MESHY-022](MESHY-022--ci-runs-the-mesh.md): the parent epic.
- [MESHY-032](MESHY-032--a-merge-applies-the-mesh.md): the apply on merge.
- [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md): the JSON the plan prints.

## Acceptance Criteria

- [ ] A pull request runs `validate` and `plan` and fails on a difference.
- [ ] A pull request from a fork runs `validate` only.
- [ ] The plan job writes its JSON to the step summary.
- [ ] The scheduled run reports drift and applies nothing.
- [ ] No step prints a share or a key.
- [ ] The workflow pins every action by commit SHA.
