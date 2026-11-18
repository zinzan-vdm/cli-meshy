---
# MESHY-022
title: CI runs the mesh
status: completed
type: epic
priority: high
tags:
    - ci
    - automation
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T15:00:00Z
---
## Overview

A pull request runs `meshy plan` and shows the difference. A merge to `main` runs `meshy unlock`, `meshy apply`, and `meshy lock`. A weekly run checks drift and renewal. This epic delivers the workflow that a mesh repository carries.

## Context / Background

The runner is the control plane, so the workflow is part of the product. It must unseal the vault from secrets, apply, and leave nothing behind. It must never run with the vault open on a pull request from a fork.

## Specification

Triggers:

| Event | Job |
|---|---|
| `pull_request` | `plan` with no vault, exit 1 on a difference |
| `push` to `main` | `apply`, with the vault unsealed from secrets |
| `schedule`, weekly | `plan`, and `apply` when a certificate is inside the window |
| `workflow_dispatch` | `plan` or `apply`, chosen by input |

Rules:

- A `pull_request` from a fork gets no secrets, so it runs `validate` only.
- The apply job unseals with `MESHY_SHARE_1` to `MESHY_SHARE_3` and runs `meshy lock` in an `always()` step.
- The plan job posts its JSON as a check summary.

## Technical Notes

- The workflow is a template in `internal/scaffold` and is written by `meshy init`.
- Actions are pinned by commit SHA.
- The SSH key comes from a repository secret and is written to a temporary file with mode 0600.
- The binary is downloaded from the meshy release, verified by checksum, and cached.

## Reference Material

- [ADR 0002](../docs/adr/0002-github-actions-is-the-primary-executor.md): CI is the executor.
- [ADR 0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md): the pull request is the gate.
- [MESHY-023](MESHY-023--a-pull-request-plans-the-mesh.md): the task.

## Acceptance Criteria

- [ ] A pull request with a difference fails the check and shows the plan.
- [ ] A pull request from a fork runs validation only and needs no secret.
- [ ] A merge to `main` applies and leaves no plaintext behind.
- [ ] The weekly run reports drift and renews inside the window.
- [ ] A failed apply fails the run and names the node.
- [ ] The lock step runs even when the apply step fails.

## Status

Both tasks of the epic are complete. [MESHY-023](MESHY-023--a-pull-request-plans-the-mesh.md) made a pull request plan the mesh, and [MESHY-032](MESHY-032--a-merge-applies-the-mesh.md) made a merge, a weekly run, and a manual run apply it.

The template that `meshy init` writes now carries both halves:

| Event | Job | Vault |
|---|---|---|
| `pull_request` from this repository | plan | closed |
| `pull_request` from a fork | validate only | no secrets |
| `push` to `main` | plan, unlock, apply, lock | shares from secrets |
| `schedule`, weekly | plan, then apply only when the plan exits 1 | shares from secrets |
| `workflow_dispatch` | the same as the weekly run | shares from secrets |

The apply job needs the pinned `nebula-cert`, because `apply` signs and renews, so the job fetches and verifies that tool before it applies.

## What is not proved yet

The workflow has not run against a real mesh repository, because this project ships no release of the CLI. The jobs are checked by tests over the template, and by the digest logic that the e2e job shares. The first mesh repository to adopt the template carries that proof, and [MESHY-043](MESHY-043--a-host-becomes-a-node.md) waits on the same run.
