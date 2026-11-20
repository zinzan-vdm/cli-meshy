---
# MESHY-047
title: The README names the checks that block a merge
status: completed
type: task
priority: normal
tags:
    - docs
    - ci
    - release
created_at: 2026-11-19T10:38:40Z
updated_at: 2026-11-19T12:44:28Z
---

## Overview

The README gains a section that names the four workflows and the check that each one reports. A repository owner reads the names and requires them on `main`. Without this list, a change that breaks a live tunnel can merge.

## Context / Background

The repository runs four workflows. `ci` proves the build and the unit tests. `e2e` proves a live mesh. `rotate` proves a live rotation. `host` proves an empty host. The last three need root, `/dev/net/tun`, and a container runtime, so each one sits in its own file.

No live check is required on `main` today. The repository states nothing about which checks block a merge, so a maintainer must read four workflow files to learn the names. [0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md) states that a pull request is the approval gate. A gate that lets a red live check merge is not a gate.

This section belongs in the README and not in [0002](../docs/conf/0002-the-mesh-workflow.md). That document describes a mesh repository. This section describes the repository that holds the tool.

## Specification

The README gains one section named `## Protect main`. It sits after `## Build` and before `## Use`.

The section holds one table. The check name is the `name` of the job in the workflow file.

| Workflow | Check | What it proves |
|---|---|---|
| `ci.yml` | `test` | the build, the vet, and the unit tests pass |
| `e2e.yml` | `live mesh` | four assertions on a live mesh of containers |
| `rotate.yml` | `live rotation` | phase 2 and phase 3 of a rotation lose no reply |
| `host.yml` | `empty host` | seven assertions on a host that holds nothing |

The section states the setting in this order: Settings, Branches, Add branch protection rule, `main`, Require status checks to pass, then the four check names.

The section states that the three live checks need root, `/dev/net/tun`, and a container runtime. That need is the reason each one has its own workflow and its own job.

## Technical Notes

- Keep the table to four rows. A required check is a check that every pull request runs. A job that a maintainer starts by hand is not a required check.
- Read the check name from the workflow file before you write the row. Do not infer it from the file name.
- Do not repeat the secrets list. [0002](../docs/conf/0002-the-mesh-workflow.md) already names the secrets that a mesh repository sets.

## Reference Material

- [0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md): the pull request is the approval gate.
- [0002](../docs/conf/0002-the-mesh-workflow.md): the workflow that a mesh repository runs, and the secrets that it needs.
- `.github/workflows/`: the four workflow files.

## Acceptance Criteria

- [x] The README holds a section named `## Protect main`.
- [x] The section lists the four workflows and the check name that each one reports.
- [x] The section names the repository setting and the four checks to select.
- [x] The section states that the three live checks need root, `/dev/net/tun`, and a container runtime.
- [x] `python3 ~/.hermes/skills/writing/write-tism/scripts/audit.py README.md` reports `total: 0`.
- [x] `beans check` passes.

## Summary of Changes

The README names the four workflows, the check that each one reports, and the setting that requires them on `main`.

### Files

- `README.md`: a `## Protect main` section sits between `## Build` and `## Use`. It holds the four-row table, the setting path, and the note on root, `/dev/net/tun`, and a container runtime.

### Decisions

- The check name comes from the `name` field of the job in each workflow file. `ci.yml` sets no `name`, so its job id `test` is the check name that GitHub reports.
- The table stays at four rows. `e2e.yml`, `rotate.yml`, and `host.yml` also accept `workflow_dispatch`, but each one runs on every pull request, so each one can block a merge.
- The section points at no secrets. `docs/conf/0002` already names them.

### Deviations

None.

### Measurements

- `gofmt -l .` silent, `go build ./...` and `go vet ./...` pass.
- 377 test results across 24 packages.
- `audit.py` reports `total: 0` for `README.md` and this ticket.
- `beans check` reports `All checks passed`.
