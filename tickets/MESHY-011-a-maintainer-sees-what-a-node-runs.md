---
id: MESHY-011
title: A maintainer sees what a node actually runs
type: task
status: todo
priority: high
parent: MESHY-008
tags: [ssh, state]
---

## Overview

`meshy` connects to a server node and reads what it actually runs: the Nebula version, the certificate, the interface, the service state, and the config hash. The plan compares that reading with the mesh file.

## Context / Background

There is no state file, so the nodes are the state. That makes this read the foundation of `plan`, and the reason `apply` can be idempotent.

## Specification

One SSH connection per node, with a bound of four in flight.

Commands run on the node, in one session:

| Fact | Source |
|---|---|
| binary version | `nebula -version` |
| service state | `systemctl is-active nebula@meshy-<mesh>` |
| certificate | `nebula-cert print -path <cert>` |
| interface present | `ip link show <interface>` |
| config hash | `sha256sum <config>` |
| authority fingerprints | `nebula-cert print -path <ca>` |

The result is a struct per node. A read that fails returns an error that names the node and the command.

`ssh` from `mesh.yml` supplies the user, host, and port. The key comes from `MESHY_SSH_KEY` or the agent. There is no password path.

## Technical Notes

- `golang.org/x/crypto/ssh` with a known hosts file, checked on every connection.
- Every command is read only. Nothing writes, restarts, or reloads.
- The certificate parse reuses the same `cert` package as issuance, so the plan compares like with like.
- A node that answers nothing within ten seconds fails the read rather than hanging the plan.

## Reference Material

- [MESHY-008](MESHY-008-see-what-a-merge-will-change.md): the parent epic.
- [ADR 0006](../docs/adr/0006-explicit-addresses-and-node-state.md): the nodes hold the state.
- [MESHY-014](MESHY-014-apply-the-config-to-a-node.md): the write side.

## Acceptance Criteria

- [ ] The read returns the version, the service state, the certificate, the interface, and the config hash.
- [ ] A node that is unreachable fails with its name and the reason.
- [ ] A node with no certificate fails with a message that says so.
- [ ] The read never writes to the node.
- [ ] Host keys are checked, and a mismatch fails the read.
- [ ] Four nodes are read at once and the rest wait.
- [ ] A test against a local SSH server returns the expected struct.
