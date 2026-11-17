---
# MESHY-013
title: A merge changes the mesh
status: todo
type: epic
priority: high
tags:
    - apply
    - ssh
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T00:45:00Z
---
## Overview

`meshy apply` makes the nodes match the mesh file. It writes configuration, renews certificates inside the window, reloads the service, and revokes a node that left the mesh. A merge to `main` runs it, so the merge is the change.

## Context / Background

Apply is the only command that writes to a node, so it carries the safety requirements. Validate first, read before write, write only what differs, and never install a binary. Renewal and revocation belong here because both are consequences of the mesh file changing.

## Specification

```
meshy apply [--mesh <dir>] [--node <name>] [--dry-run]
```

| Step | Behaviour |
|---|---|
| 1 | validate, and stop on any problem |
| 2 | read every server node |
| 3 | issue or renew a certificate that is missing or inside the window |
| 4 | render the config and write it only when the hash differs |
| 5 | reload the unit only when the config or the certificate changed |
| 6 | push the blocklist when a node left the mesh |
| 7 | print a summary, one line per node |

`--node` limits the run to one node. `--dry-run` is `plan` and writes nothing.

Apply never installs a binary and never changes the pinned version.

## Technical Notes

- Every write is idempotent, so a second run changes nothing and prints no writes.
- A node that fails does not stop the others. The exit code is 1 and the summary names the failures.
- The certificate and key are written with mode 0600, owned by the node user.

## Delivery

The vertical is one pull request. Each commit carries one ticket, so a reviewer reads the change one ticket at a time.

| Order | Ticket | The change |
|---|---|---|
| 1 | [MESHY-007](MESHY-007--a-node-gets-its-certificate.md) | a node with no certificate receives one |
| 2 | [MESHY-014](MESHY-014--apply-the-config-to-a-node.md) | the config and the certificate reach the node |
| 3 | [MESHY-015](MESHY-015--renew-a-certificate.md) | a certificate inside the window is renewed |
| 4 | [MESHY-016](MESHY-016--remove-a-node-from-the-mesh.md) | a node that left is revoked on every node |
| 5 | [MESHY-042](MESHY-042--apply-reports-one-line-per-node.md) | the command runs every node and reports each |

MESHY-007 belongs to [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md), and it is that epic's last open criterion. It lands first in this pull request, because a certificate has to exist before a node can be given one.

[MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md) is not in this pull request. It moved to [MESHY-043](MESHY-043--a-host-becomes-a-node.md), which turns an empty host into a node. That work needs the binary install, and both are reviewable together.

### Proof

The remote side is reached through one narrow interface, and the tests supply a fake that records the commands and the bytes it receives. So the vertical is proved without a node, and the suite still runs on a laptop.

One test is gated on a real sshd. It is skipped unless `MESHY_TEST_SSHD` names an address, which keeps CI free of a daemon.

## Reference Material

- [ADR 0002](../docs/adr/0002-github-actions-is-the-primary-executor.md): CI is the executor.
- [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md): the read side.
- [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md): the separate command for binaries.

## Acceptance Criteria

- [ ] A merge with no change writes nothing and exits 0.
- [ ] A changed rule rewrites only the destination node config.
- [ ] A certificate inside the window is renewed and the node reloads.
- [ ] A node that leaves the mesh is revoked and its fingerprint reaches every node.
- [ ] A mesh that fails validation writes nothing.
- [ ] A failure on one node still applies the others, and the exit code is 1.
- [ ] No apply installs or changes a binary.
