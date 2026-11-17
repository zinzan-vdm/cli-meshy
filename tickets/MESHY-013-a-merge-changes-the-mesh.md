---
id: MESHY-013
title: A merge changes the mesh
type: epic
status: todo
priority: high
tags: [apply, ssh]
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

## Reference Material

- [ADR 0002](../docs/adr/0002-github-actions-is-the-primary-executor.md): CI is the executor.
- [MESHY-012](MESHY-012-a-maintainer-sees-the-plan.md): the read side.
- [MESHY-021](MESHY-021-the-nodes-run-the-pinned-release.md): the separate command for binaries.

## Acceptance Criteria

- [ ] A merge with no change writes nothing and exits 0.
- [ ] A changed rule rewrites only the destination node config.
- [ ] A certificate inside the window is renewed and the node reloads.
- [ ] A node that leaves the mesh is revoked and its fingerprint reaches every node.
- [ ] A mesh that fails validation writes nothing.
- [ ] A failure on one node still applies the others, and the exit code is 1.
- [ ] No apply installs or changes a binary.
