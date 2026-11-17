---
tags:
  - config
  - ci
---

# Mesh Workflow Variables

The values that the workflow in a mesh repository reads from outside the file. `meshy init` writes the file to `.github/workflows/meshy.yml`.

A maintainer sets both secrets in the repository settings, under Secrets and variables, then Actions. The `plan` job fails with a named error when a secret is absent.

## `MESHY_KNOWN_HOSTS`

- Type: string
- Required: yes
- Source: secret store, repository secret
- Description: The known hosts file that every node host key is checked against.
- Constraints: One known_hosts line for each server node, in OpenSSH format. The host and port must match the `ssh` block of the node.

The job writes the value to `$RUNNER_TEMP/meshy/known_hosts` with mode 0600. `meshy` reads that path from the environment variable of the same name. The step passes the secret under the alias `HOSTS`, so the two uses of the name cannot collide.

## `MESHY_SSH_PRIVATE_KEY`

- Type: string
- Required: yes
- Source: secret store, repository secret
- Description: The private key that reads the server nodes.
- Constraints: An OpenSSH private key with no passphrase. The user must match the `ssh.user` block of each node.

The job writes the value to `$RUNNER_TEMP/meshy/id_ed25519` with mode 0600. `meshy` reads that path from `MESHY_SSH_KEY`. The job removes the directory in an `always()` step.

## Triggers and jobs

| Event | Jobs |
|---|---|
| `pull_request` from this repository | `validate`, `plan` |
| `pull_request` from a fork | `validate` |
| `schedule`, Monday 03:00 UTC | `validate`, `plan` |
| `workflow_dispatch` | `validate`, `plan` |

A fork run holds no secrets, so it cannot read a node. The `plan` job names the fork in its `if` condition.

The scheduled run reports drift and certificate renewal dates. It applies nothing, because the workflow holds no apply job.

## What the plan step writes

The job writes the JSON of `meshy plan --json` to `$GITHUB_STEP_SUMMARY`. The reviewer reads it on the run page. The summary holds a heading, one line that names the outcome, and the JSON in a fenced block.

| Exit code | Summary line | Job |
|---|---|---|
| 0 | The nodes match mesh.yml. | passes |
| 1 | A merge changes the mesh. | passes |
| 2 | meshy plan exited 2 and produced no plan. | fails |

Exit code 1 passes the job. The difference is the report that the reviewer approves, and a red check would hide it. See [0006](../patterns/0006-a-difference-is-a-report.md).

## The CLI pin

The workflow reads the CLI from the commit in `MESHY_CLI_REF`, which sits at the top of the file. `go install` builds `github.com/zinzan-vdm/cli-meshy/cmd/meshy` at that commit.

This path is not proven on a runner. The module must be readable from the runner, and a private module needs a token. The workflow sets no token. The release download is also unproven, because the project ships no release yet.

To move a mesh to a newer control plane, change `MESHY_CLI_REF` in a pull request. The reviewer sees the change.
