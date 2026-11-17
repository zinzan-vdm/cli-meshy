---
# MESHY-011
title: A maintainer sees what a node actually runs
status: completed
type: task
priority: high
tags:
    - ssh
    - state
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T21:45:05Z
parent: MESHY-008
---

## Overview

`meshy` connects to a server node and reads what it actually runs: the Nebula version, the certificate, the interface, the service state, and the config hash. The plan compares that reading with the mesh file.

## Context / Background

There is no state file, so the nodes are the state. That makes this read the foundation of `plan`, and the reason `apply` can be idempotent.

Part of [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): a maintainer can see what a merge will change.

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

- [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): the parent epic.
- [ADR 0006](../docs/adr/0006-overlay-addresses-are-explicit.md): the nodes hold the state.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write side.

## Acceptance Criteria

- [x] The read returns the version, the service state, the certificate, the interface, and the config hash.
- [x] A node that is unreachable fails with its name and the reason.
- [x] A node with no certificate fails with a message that says so.
- [x] The read never writes to the node.
- [x] Host keys are checked, and a mismatch fails the read.
- [x] Four nodes are read at once and the rest wait.
- [x] A test against a local SSH server returns the expected struct.

## Summary of Changes

- `internal/node/state.go`: `State`, `Cert`, and the four parsers. `ParseCert` reads the JSON form, so no certificate package is needed.
- `internal/node/read.go`: `Read`, `ReadAll`, `Commands`, `Dir`, `Unit`, and the bound of four.
- `internal/node/state_test.go`, `internal/node/read_test.go`: 15 cases, including the read-only command list and the bound on work in flight.
- `internal/sshx/client.go`: `Dial`, `Run`, `FromNode`, and the two ways to authenticate.
- `internal/sshx/hostkey.go`: `HostKeys`, which has no path that skips the check.
- `internal/sshx/hostkey_test.go`, `internal/sshx/read_test.go`: 6 cases, one of them a full read against a local SSH server.

Two changes from the specification.

The certificate is read with `nebula-cert print -json`, not by importing `github.com/slackhq/nebula/cert`. That module raises the Go floor to 1.26 and pulls in gvisor and the Windows WireGuard driver. The JSON form gives the same fields, so the plan can compare like with like. Issuance in MESHY-007 still needs the real signing code, and that decision belongs there.

`golang.org/x/crypto` is pinned to v0.31.0, the last release that builds on Go 1.22.

The read has no command of its own yet. MESHY-012 exposes it through `meshy plan`.
