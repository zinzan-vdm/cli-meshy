---
tags:
  - ci
  - review
---

# Pin every action to a commit SHA

Every action in a workflow in this repository, and in the workflow that `meshy init` writes, names a full 40 character commit SHA. The release tag sits in a comment on the same line.

A tag can move. A mesh workflow holds secrets that reach every node, so an action that changes under a tag changes the control plane without a review.

## Enforcement

`TestWorkflowPinsEveryAction` in `internal/scaffold` parses the generated workflow. It rejects a `uses:` value that does not end in `@` and 40 lowercase hex characters.

## See also

- [0006](0006-a-difference-is-a-report.md): a difference is a report.
- [ADR 0002](../adr/0002-github-actions-is-the-primary-executor.md): CI is the executor.
