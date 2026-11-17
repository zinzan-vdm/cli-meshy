---
id: MESHY-022
title: CI runs the mesh
type: epic
status: todo
priority: high
tags: [ci, automation]
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
- [MESHY-023](MESHY-023-a-pull-request-plans-the-mesh.md): the task.

## Acceptance Criteria

- [ ] A pull request with a difference fails the check and shows the plan.
- [ ] A pull request from a fork runs validation only and needs no secret.
- [ ] A merge to `main` applies and leaves no plaintext behind.
- [ ] The weekly run reports drift and renews inside the window.
- [ ] A failed apply fails the run and names the node.
- [ ] The lock step runs even when the apply step fails.
