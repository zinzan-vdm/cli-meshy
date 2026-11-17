---
# MESHY-007
title: A node gets its certificate
status: completed
type: task
priority: high
tags:
    - ca
    - certificates
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T23:34:00Z
parent: MESHY-004
---
## Overview

`meshy issue <node>` mints a certificate for one node, records it in the vault, and leaves the mesh otherwise unchanged. It is the command that turns a name in `mesh.yml` into an identity that other nodes trust.

## Context / Background

Adding a node to `mesh.yml` and merging is the approval step. Issuance is the mechanical step that follows, and it can run unattended in CI because the review already happened.

Issuance is idempotent: a node that already holds a valid certificate gets nothing new unless the certificate is close to expiry or `--force` is given.

Part of [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the vault seals and unseals the mesh keys.

## Specification

```
meshy issue <node> [--force] [--all] [--mesh <dir>]
```

| State | Behaviour |
|---|---|
| no certificate in the vault | sign one, store it, print the fingerprint |
| certificate valid for more than `renew_before` | print `already valid until <date>`, change nothing |
| certificate inside the window | sign a new one with the same address, store it |
| `--force` | sign a new one |
| unknown node | exit 1, `node "x" is not in mesh.yml` |
| vault closed | exit 1, `the vault is locked, run meshy unlock` |

The certificate carries the node name, its `ip`, and its `groups`. The lifetime comes from `certificates.<kind>`.

## Technical Notes

- The certificate and key are stored in the vault under the node name.
- The node key is generated on the control host and never leaves it except inside a bundle.
- `issue` runs `validate` first and refuses on problems.
- A test issues twice and asserts that the second run changes nothing.

## Reference Material

- [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the parent epic.
- [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md): the plan that reports an expiring certificate.
- [ADR 0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md): the review is the approval.

## Acceptance Criteria

- [x] The certificate names the node, its address, and its groups.
- [x] The lifetime matches `certificates.server` or `certificates.user`.
- [x] A second `issue` on a valid certificate changes nothing.
- [x] An expiring certificate is replaced without `--force`.
- [x] An unknown node exits 1 with the node name in the message.
- [x] A locked vault exits 1 and says so.
- [x] The vault record holds the certificate and the key for the node.

## Summary of Changes

`meshy issue` mints one certificate from the active authority. It stores the certificate and the key in the vault under the node name, and it leaves a valid certificate alone. The vault keeps the shares that a person already holds.

### Files

- `internal/issue/issue.go`: `Due`, `Sign`, `Print`, and `Expiry`. `Sign` runs the pinned `nebula-cert` in a temporary directory and returns the certificate, the key, the expiry, and the fingerprint.
- `internal/issue/issue_test.go`: 8 tests for the renewal window, the arguments, the returned facts, and three refusals.
- `internal/vault/node.go`: `Certificate` and `SetCertificate`, so one place owns the `<name>.crt` and `<name>.key` names.
- `internal/vault/vault.go`: `Read` and `Write` for an open vault, `ErrLocked`, and `identityFrom`. `Unlock` now also writes the key that seals the vault into `.meshy/age.identity`.
- `internal/vault/open_test.go`: 7 tests. The first proves that a vault sealed by `ca init` still opens with its own shares after a node entry is added.
- `internal/vault/record.go`: `Encode` refuses the name of the file that seals the vault.
- `cmd/meshy/issue.go`: `cmdIssue`, the per-node decision, the network form, and the lifetime by kind.
- `cmd/meshy/issue_test.go`: 13 tests against a stub binary that records its arguments.
- `cmd/meshy/issue_integration_test.go`: the chain proof with the real binary, run when `MESHY_NEBULA_CERT` names one.
- `cmd/meshy/main.go`: the `issue` case, one usage line, and `issue` removed from the planned list.
- `docs/patterns/0005-the-vault-record-holds-one-file-per-name.md`, `docs/patterns/0006-a-write-to-the-vault-seals-it-with-the-key-that-opened-it.md`, `docs/patterns/TAGS`: the record layout and the re-seal rule.

### Decisions

- The record keeps its flat shape. A node is `<node>.crt` and `<node>.key`, as the authority is `ca-1.crt` and `ca-1.key`. A vault that `ca init` wrote today therefore needs no conversion, and a test holds that.
- A write seals `secrets.age` with the key that opened the vault, not with `vault.Seal`. `Seal` makes a new key, and the five shares are printed once, so a new key on every write would lose the vault. The open vault now carries that key in `.meshy/age.identity`, mode 0600, and `lock` removes it with the directory.
- `vault.Write` writes the open vault first. It replaces the sealed file last, through a temporary name, so a failure leaves the sealed file that a person already has.
- The lifetime goes to the tool in hours, as `ca init` does. One year is 365 days, so a server certificate asks for `8760h` and a user certificate of 90 days asks for `2160h`.
- The address goes to the tool as `<ip>/<prefix>`, because v1.12.0 requires `-networks` and takes a network.
- `--all` walks every node in `mesh.yml` in file order and applies the same table to each. A node argument together with `--all` is a usage error, because the two name different sets of nodes.

### Deviations

- A locked vault exits 1, as the specification says. The exit code contract in `cmd/meshy/plan.go` gives 2 to a run that never started, and a locked vault is that. The ticket is the spec, so it wins here. A validation failure, a missing binary, and an unreadable stored certificate exit 2.
- `--all` is in the synopsis but not in the state table. It issues for every node and prints one line per node. The message for a valid certificate keeps the exact words `already valid until <date>` after a `<node>: ` prefix, so a batch reads.
- A certificate in the vault that the tool cannot read stops the run with exit 2. The table has no state for it, and replacing a certificate that a person may still trust without saying so is worse than stopping.
- The run stops at the first node that fails. Certificates signed earlier in that run are not written, because the sealed file is written once at the end.
- The open vault gains one file, `age.identity`. MESHY-005 defines the directory and its modes, not its file list, and `lock` still removes all of it. No command prints the file.
- A note for MESHY-021: `writeRecord` clears `.meshy` before it writes the record, so a binary cache in `.meshy/bin/<version>/` would not survive an `unlock` or a `write`. `ca.Find` already looks there. The order of `install` and `unlock` has to settle that, and this ticket does not change it.
