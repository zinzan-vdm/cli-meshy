# Tickets

One file per ticket. An epic holds tasks. A subtask is a checklist item inside a task body.

## Status

| Status | Meaning |
|---|---|
| `draft` | Written, not scheduled. |
| `todo` | Scheduled, not started. |
| `in-progress` | Someone is working on it. |
| `completed` | Acceptance criteria pass. The body has a Summary of Changes. |
| `scrapped` | The work will not be done. The body has Reasons for Scrapping. |

## Epics

| ID | Outcome | Status |
|---|---|---|
| [MESHY-001](MESHY-001-describe-and-validate-a-mesh.md) | A maintainer can describe and validate a mesh. | completed |
| [MESHY-004](MESHY-004-the-vault-seals-and-unseals.md) | The vault seals and unseals the mesh keys. | todo |
| [MESHY-008](MESHY-008-see-what-a-merge-will-change.md) | A maintainer can see what a merge will change. | todo |
| [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md) | A merge changes the mesh. | todo |
| [MESHY-017](MESHY-017-join-without-a-terminal.md) | A person joins without a terminal. | todo |
| [MESHY-020](MESHY-020-binaries-match-the-pin.md) | Binaries on the nodes match the pin. | todo |
| [MESHY-022](MESHY-022-ci-runs-the-mesh.md) | CI plans on a pull request and applies on merge. | todo |
| [MESHY-024](MESHY-024-the-trust-anchor-rotates.md) | The trust anchor rotates without downtime. | todo |
| [MESHY-026](MESHY-026-devices-without-nebula.md) | Devices that cannot run Nebula reach the mesh. | draft |

## Tasks

| ID | Task | Parent | Status |
|---|---|---|---|
| [MESHY-002](MESHY-002-scaffold-a-mesh-repository.md) | Scaffold a mesh repository | 001 | completed |
| [MESHY-003](MESHY-003-validate-a-mesh-file.md) | Validate a mesh file | 001 | completed |
| [MESHY-005](MESHY-005-open-and-close-the-vault.md) | Open and close the vault | 004 | todo |
| [MESHY-006](MESHY-006-create-the-certificate-authority.md) | Create the certificate authority | 004 | todo |
| [MESHY-007](MESHY-007-issue-a-certificate.md) | Issue a certificate for a node | 004 | todo |
| [MESHY-009](MESHY-009-render-the-nebula-config.md) | Render the Nebula config for one node | 008 | todo |
| [MESHY-010](MESHY-010-expand-rules-into-a-firewall.md) | Expand rules into a per-node firewall | 008 | todo |
| [MESHY-011](MESHY-011-read-live-state-over-ssh.md) | Read live state from a server node | 008 | todo |
| [MESHY-012](MESHY-012-report-the-plan.md) | Report the plan and its exit code | 008 | todo |
| [MESHY-014](MESHY-014-write-config-and-reload.md) | Write config and reload a server node | 013 | todo |
| [MESHY-015](MESHY-015-renew-a-certificate.md) | Renew a certificate inside the window | 013 | todo |
| [MESHY-016](MESHY-016-revoke-and-blocklist.md) | Revoke and blocklist a removed node | 013 | todo |
| [MESHY-018](MESHY-018-build-a-bundle.md) | Build a bundle for a user node | 017 | todo |
| [MESHY-019](MESHY-019-ship-a-platform-installer.md) | Ship a platform installer | 017 | todo |
| [MESHY-021](MESHY-021-install-a-pinned-release.md) | Install a pinned Nebula release | 020 | todo |
| [MESHY-023](MESHY-023-ci-plans-and-applies.md) | Plan on a pull request and apply on merge | 022 | todo |
| [MESHY-025](MESHY-025-rotate-the-certificate-authority.md) | Rotate the certificate authority | 024 | todo |
| [MESHY-027](MESHY-027-bootstrap-a-new-server-node.md) | Bootstrap a new server node | 013 | todo |
