---
tags:
  - ci
  - plan
---

# A difference is a report

In the mesh workflow, the `plan` job fails only when `meshy plan` produced no plan. A difference is the report that the reviewer approves, so it passes the job.

`meshy plan` exits 1 when the mesh holds a difference, and 2 when no plan could be produced. A difference is the expected outcome of a pull request that changes a node. A red check would hide the change behind a failed run, and a reviewer would learn to bypass it. Exit 2 means the tool could not read the mesh or a node, so a reviewer must not read it as a plan. That run fails.

The text of the summary states which of the three codes the run got.

## Enforcement

`TestPlanJobFailsOnlyWithoutAPlan` in `internal/scaffold` reads the generated workflow. It asserts a check on exit code 2 and no return of the raw exit code.

## See also

- [0007](0007-pin-every-action-to-a-commit-sha.md): pin every action to a commit SHA.
