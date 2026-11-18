---
tags:
  - config
  - ci
---

# Mesh Workflow Variables

The values that the workflow in a mesh repository reads from outside the file. `meshy init` writes the file to `.github/workflows/meshy.yml`.

A maintainer sets five secrets in the repository settings, under Secrets and variables, then Actions. The `plan` job fails with a named error when the ssh secrets are absent. The `apply` job fails with a named error when the ssh secrets or a share are absent.

## `MESHY_KNOWN_HOSTS`

- Type: string
- Required: yes
- Source: secret store, repository secret
- Description: The known hosts file that every node host key is checked against.
- Constraints: One known_hosts line for each server node, in OpenSSH format. The host and port must match the `ssh` block of the node.

The `plan` job and the `apply` job each write the value to `$RUNNER_TEMP/meshy/known_hosts` with mode 0600. `meshy` reads that path from the environment variable of the same name. The step passes the secret under the alias `HOSTS`, so the two uses of the name cannot collide.

## `MESHY_SSH_PRIVATE_KEY`

- Type: string
- Required: yes
- Source: secret store, repository secret
- Description: The private key that reads the server nodes.
- Constraints: An OpenSSH private key with no passphrase. The user must match the `ssh.user` block of each node.

The job writes the value to `$RUNNER_TEMP/meshy/id_ed25519` with mode 0600. `meshy` reads that path from `MESHY_SSH_KEY`. Each job removes the directory in an `always()` step.

## `MESHY_SHARE_1`, `MESHY_SHARE_2`, `MESHY_SHARE_3`

- Type: string
- Required: yes, all three
- Source: secret store, repository secret
- Description: One Shamir share of the vault key. Three shares open the vault.
- Constraints: The shares of one vault. A share from another vault fails the open.

The `apply` job passes the shares to `meshy unlock --from-env` as the environment variables `MESHY_SHARE_1` to `MESHY_SHARE_3`. The command reads the environment and writes no share to the log. The step names a missing share with `::error::` and exits 1.

Only the `apply` job holds a share. A pull request never reaches that job, so a fork gets no share and no authority key.

## Triggers and jobs

| Event | Jobs |
|---|---|
| `pull_request` from this repository | `validate`, `plan` |
| `pull_request` from a fork | `validate` |
| `push` to `main` | `validate`, `apply` |
| `schedule`, Monday 03:00 UTC | `validate`, `plan`, `apply` |
| `workflow_dispatch` | `validate`, `plan`, `apply` |

A fork run holds no secrets, so it cannot read a node. The `plan` job names the fork in its `if` condition. The `apply` job names only `push`, `schedule`, and `workflow_dispatch`, so no pull request reaches the vault.

## How the apply job decides

The apply job runs its own `meshy plan --json` before it unseals the vault. A plan reads the certificates on the nodes, so it reports a renewal with a closed vault. The job applies when one of these is true:

1. The event is a `push` to `main`. The review approved the plan, so the merge applies it.
2. The plan exits 1. That code means at least one difference, and a renewal is a certificate difference, so one test covers drift and renewal. The gate reads the exit code, so the job needs no JSON reader.

A weekly run that finds a match applies nothing, and it never opens the vault. See [0019](../patterns/0019-the-vault-opens-only-for-a-change.md).

## What the apply job writes

The `plan` step writes the JSON of `meshy plan --json` to `$GITHUB_STEP_SUMMARY`, with one line that names the decision. The `apply` step writes the JSON of `meshy apply --json` to the same summary, so the run page shows the plan and the result.

| Step | Exit code | Job |
|---|---|---|
| `plan` | 0 or 1 | passes, with or without a change |
| `plan` | 2 | fails, because no plan exists |
| `apply` | 0 | passes |
| `apply` | 1 or 2 | fails, and the report names each node |

A difference passes the `plan` step. The difference is the report that the reviewer approves, and a red check would hide it. See [0008](../patterns/0008-a-difference-is-a-report.md).

## The tool that signs

`meshy apply` signs a missing certificate and renews one inside `certificates.renew_before`. It looks for `nebula-cert` in `.meshy/bin/<version>/<os>-<arch>/`, then on `PATH`. See `internal/ca`.

The `apply` job downloads the Nebula release that `mesh.yml` pins. It reads the version from `mesh.yml`. It fetches `SHASUM256.txt` and `nebula-linux-amd64.tar.gz` from the URL in `internal/release`. It checks the archive and the member against the published digests before it uses the tool. It then places the tool at `.meshy/bin/<version>/linux-amd64/nebula-cert`, which is the first path that `ca.Find` reads. The steps follow [0012](../patterns/0012-the-release-is-verified-before-a-node-sees-it.md).

The step runs after `meshy unlock`, because `unlock` refuses a `.meshy` directory that already holds a file. `meshy lock` removes the whole directory, so the tool leaves the runner with the vault.

The runner is `ubuntu-latest`, which is `linux-amd64`. A runner of another architecture does not use this step.

## The CLI pin

The workflow reads the CLI from the commit in `MESHY_CLI_REF`, which sits at the top of the file. `go install` builds `github.com/zinzan-vdm/cli-meshy/cmd/meshy` at that commit.

This path is not proven on a runner. The module must be readable from the runner, and a private module needs a token. The workflow sets no token. The release download is also unproven, because the project ships no release yet.

The `apply` job is unproven on a runner. No run has applied a mesh from a repository secret, and no run has opened a vault from three shares. The template holds the steps, the order, and the guards. A first run on a real mesh proves or corrects them.

To move a mesh to a newer control plane, change `MESHY_CLI_REF` in a pull request. The reviewer sees the change.
