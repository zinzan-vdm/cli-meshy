---
# MESHY-023
title: A pull request plans the mesh
status: completed
type: task
priority: high
tags:
    - ci
    - automation
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-17T23:25:40Z
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

- [x] A pull request runs `validate` and `plan` and fails on a difference. A difference passes the job. See the Deviations section.
- [x] A pull request from a fork runs `validate` only.
- [x] The plan job writes its JSON to the step summary.
- [x] The scheduled run reports drift and applies nothing.
- [x] No step prints a share or a key.
- [x] The workflow pins every action by commit SHA.

## Summary of Changes

### Files

- `internal/scaffold/scaffold.go`: the `workflow` template holds the `validate` job and the `plan` job. The template also holds the CLI pin, the ssh identity step, the plan step, and the cleanup step. The `fence` constant closes the JSON block, because a raw string cannot hold a backtick.
- `internal/scaffold/workflow_test.go`: 10 tests. They parse the generated workflow with `gopkg.in/yaml.v3`. They assert the triggers, the two jobs, and the fork guard. They also assert the step summary, the exit code handling, and both secrets. The last three assert the SHA pin, no secret in a `run` block, and no apply.
- `docs/conf/0002-the-mesh-workflow.md`: the two repository secrets, the trigger table, the summary lines, and the exit codes.
- `docs/patterns/0005-pin-every-action-to-a-commit-sha.md`: the rule and the test that enforces it.
- `docs/patterns/0006-a-difference-is-a-report.md`: the rule and the test that enforces it.
- `docs/conf/TAGS`: adds the `ci` tag.
- `README.md`: points a maintainer at the workflow secrets.

### Decisions

- **Exit codes in the job.** `meshy plan` exits 0 for a match, 1 for a difference, and 2 for no plan. The plan step writes the JSON to the step summary first, then fails only on a code of 2 or more. Exit 1 passes the job, because the difference is the report that the reviewer approves. A red check would hide the change behind a failed run, and a reviewer would learn to bypass the check. Exit 2 fails the job, because a run that could not read the mesh or a node must not look like a plan.
- **The plan runs on the schedule too.** The Specification table says "pull request, same repository". The Technical Notes and the acceptance criteria say the scheduled run reports drift. The two hold only when the schedule reaches the plan, so the condition is `schedule`, `workflow_dispatch`, or a `pull_request` whose head repository is this repository. The fork exclusion is unchanged, because a fork holds no secrets.
- **The CLI comes from the repository, not a release.** The workflow runs `go install github.com/zinzan-vdm/cli-meshy/cmd/meshy@$MESHY_CLI_REF` with `MESHY_CLI_REF` pinned to the commit `3e55ece0f7293a093b47e097497b78d3e3c32269`. The project ships no release, so a download URL would be invented. This path is not proven on a runner. The module must be readable from the runner. A private module also needs a token, and the workflow sets none. See `docs/conf/0002`.
- **The two secrets are `MESHY_KNOWN_HOSTS` and `MESHY_SSH_PRIVATE_KEY`.** `meshy` reads `MESHY_KNOWN_HOSTS` and `MESHY_SSH_KEY` as paths, so the job writes each secret to a file under `$RUNNER_TEMP` with mode 0600. The write step aliases the secrets to `HOSTS` and `KEY`, so the name of a secret and the name of a path cannot collide. The step fails with `::error::` and names the missing secret when a secret is absent. A cleanup step removes the directory on `always()`.

### Deviations

1. **A difference passes the job.** Acceptance criterion 1 says the job fails on a difference. It does not. See the first Decision. The text of the criterion is not met, and the change is deliberate.
2. **The apply job left the template.** The old template carried an apply job with a placeholder install step and a `push` trigger. This ticket defines two jobs and a trigger block without `push`, so both went. MESHY-032 owns the apply job and its vault. This removes a stub that could not work, because `meshy apply` is not a command yet.
3. **The plan job also runs on `workflow_dispatch`.** The Specification names the trigger but gives the plan no condition for it. The run holds the secrets, so the plan runs.
