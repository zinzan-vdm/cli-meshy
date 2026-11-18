---
tags:
  - certificates
  - rotation
  - ci
---

# A due rotation is information, not drift

`meshy plan` reports the age of the active authority and names the phase that comes next. A due rotation does not change the exit code, and it never fails a pull request.

Red means that a change is wrong. A rotation is a calendar fact. An exit code of 1 would appear on the day the authority reaches `ca.rotate_at`. That mark would sit on every open pull request, including the ones that do not touch the mesh. A red mark that appears for a reason outside the change teaches people to ignore red.

So the plan step prints a `::warning` annotation, and the merge path stays clear. The JSON carries `due` for any other reader of the report.

## Considered options

An exit code of 1 for a due rotation was rejected. It blocks unrelated work and it competes with the code that reports real drift.

A failing scheduled job was also rejected, for now. The weekly run applies drift, and a rotation is a deliberate two-phase change that a human starts.

## Consequences

Nothing escalates a due rotation on its own. The warning sits in the run that plans, and a person acts on it.

Revisit trigger: a rotation that stays due across two weekly cycles needs a louder channel. The scheduled job may then fail, because it reaches an inbox and never blocks a pull request.

## See also

- [0002](0002-github-actions-is-the-primary-executor.md): GitHub Actions is the primary executor.
- [0023](../patterns/0023-the-plan-reads-the-authority-from-the-bundle.md): the plan reads the authority from the bundle.
