---
# MESHY-049
title: The ticket set passes the checks its tickets claim
status: completed
type: task
priority: normal
tags:
    - tickets
    - ci
    - docs
created_at: 2026-11-20T19:32:25Z
updated_at: 2026-11-20T19:32:25Z
---

## Overview

Every ticket in the set is closed, and every ticket uses the section names that `use-tickets` defines. The `test` check runs `beans check`, so a ticket that breaks the set stops a merge.

## Context / Background

Two records were stale after the merge of PR #27 and PR #28. MESHY-048 held `status: in-progress`, although its Summary of Changes was written and every acceptance criterion was ticked. MESHY-041 named its closing section `## Why this was scrapped`, but `use-tickets` names that section `## Reasons for Scrapping`. `beans check` passes on both, so no check reported either one.

Every ticket names `beans check` as an acceptance criterion. No workflow ran it. A pull request could merge with a broken ticket set, and the criterion would still read `[x]`.

## Specification

- MESHY-048 has `status: completed`.
- MESHY-041 holds a section named `## Reasons for Scrapping`.
- `ci.yml` runs `beans check` in the `test` job, after the test step and before the scaffold step.
- The step pins the tool with `go run github.com/hmans/beans@v0.4.2 check`. The pin matches the version that the maintainer runs.
- The README names the command that installs the tool, next to the paragraph that describes the tickets.
- The check name stays `test`. The four rows in `## Protect main` do not change.

## Technical Notes

- Use `go run` with a version. `go install` writes the binary to `GOPATH/bin`, so the step then depends on the `PATH` that `setup-go` sets.
- Pin the version. An unpinned tool can fail a pull request that changed no ticket.
- The write-tism audit stays a local gate. Its script lives in a personal skill directory, so a workflow cannot reach it. Do not copy the script into the repository.

## Reference Material

- `use-tickets`: the section names and the status values.
- [MESHY-033](MESHY-033--the-ticket-set-follows-the-standard.md): the ticket set follows the standard.
- [MESHY-047](MESHY-047--the-readme-names-the-checks-that-block-a-merge.md): the README names the checks that block a merge.
- `.github/workflows/ci.yml`: the `test` job.

## Acceptance Criteria

- [x] MESHY-048 has `status: completed`.
- [x] MESHY-041 holds a section named `## Reasons for Scrapping`.
- [x] `ci.yml` runs `beans check` in the `test` job.
- [x] The README names the command that installs Beans.
- [x] `beans check` passes.
- [x] `audit.py` reports `total: 0` for the changed markdown files.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass.

## Summary of Changes

The ticket set closes, and the check that every ticket claims now runs on every pull request.

### Files

- `.beans/MESHY-048--phase-2-files-the-kept-pair-under-the-authority-that-signed-it.md`: `status` is `completed`.
- `.beans/MESHY-041--a-new-node-gets-its-certificate.md`: the closing section is named `## Reasons for Scrapping`.
- `.github/workflows/ci.yml`: the `test` job runs `go run github.com/hmans/beans@v0.4.2 check` after the test step.
- `README.md`: the ticket paragraph names the command that installs Beans, and states that CI runs the check.

### Decisions

- The step uses `go run` with a version, not `go install`. The binary then does not depend on the `PATH` that the runner sets.
- The version is pinned to `v0.4.2`, the release that the maintainer runs.
- The write-tism audit stays local. The script sits in a personal skill directory, so a workflow cannot reach it.
- The check name stays `test`, because the step joins the job that already exists. The `## Protect main` table does not change.

### Deviations

None.

### Measurements

- `go run github.com/hmans/beans@v0.4.2 check` reports `All checks passed`.
- `audit.py` reports `total: 0` for the four changed markdown files.
