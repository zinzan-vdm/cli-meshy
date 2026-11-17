---
id: MESHY-007
title: Issue a certificate for a node
type: task
status: todo
priority: high
parent: MESHY-004
tags: [ca, certificates]
---

## Overview

`meshy issue <node>` mints a certificate for one node, records it in the vault, and leaves the mesh otherwise unchanged. It is the command that turns a name in `mesh.yml` into an identity that other nodes trust.

## Context / Background

Adding a node to `mesh.yml` and merging is the approval step. Issuance is the mechanical step that follows, and it can run unattended in CI because the review already happened.

Issuance is idempotent: a node that already holds a valid certificate gets nothing new unless the certificate is close to expiry or `--force` is given.

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

- [MESHY-004](MESHY-004-the-vault-seals-and-unseals.md): the parent epic.
- [MESHY-012](MESHY-012-report-the-plan.md): the plan that reports an expiring certificate.
- [ADR 0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md): the review is the approval.

## Acceptance Criteria

- [ ] The certificate names the node, its address, and its groups.
- [ ] The lifetime matches `certificates.server` or `certificates.user`.
- [ ] A second `issue` on a valid certificate changes nothing.
- [ ] An expiring certificate is replaced without `--force`.
- [ ] An unknown node exits 1 with the node name in the message.
- [ ] A locked vault exits 1 and says so.
- [ ] The vault record holds the certificate and the key for the node.
