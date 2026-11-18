---
# MESHY-004
title: The vault seals and unseals the mesh keys
status: completed
type: epic
priority: high
tags:
    - vault
    - keys
    - ca
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T09:30:00Z
---
## Overview

The mesh keys live in the repository and never in plaintext. This epic delivers the vault, the certificate authority, and certificate issuance. When it is complete, a maintainer can seal a vault, open it, and mint a certificate for a node.

## Context / Background

The pipeline issues certificates and regenerates bundles, so it needs the authority key and the node keys. Those keys must travel with the repository, which means they must be encrypted at rest. Five shares seal the vault and any three open it, so no single secret holds the mesh.

`issue` mints identity, so it needs the authority key. `apply` needs the key only when it signs a certificate that is missing or inside the renewal window, which is [MESHY-015](MESHY-015--renew-a-certificate.md).

## Specification

Three commands:

```
meshy ca init
meshy unlock [--from-env]
meshy lock
meshy issue <node> [--force]
```

`ca init` creates `ca-1`, writes `secrets.age`, and prints the five shares once.

`unlock` writes plaintext to a temporary directory that the process removes. `--from-env` reads the shares from `MESHY_SHARE_1` to `MESHY_SHARE_5`.

`issue` mints a certificate for one node and records it in the vault.

## Technical Notes

- Encryption is `filippo.io/age`. Shares are Shamir parts of the file key.
- Certificate work runs the pinned `nebula-cert`, which [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) chose over importing the Nebula module. The tool comes from `.meshy/bin/<version>/` or from `PATH`.
- The build needs no Go module from Nebula. This note corrects the plan that this epic started with.

## Reference Material

- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault is sealed with age and Shamir shares.
- [ADR 0001](../docs/adr/0001-nebula-is-the-data-plane.md): Nebula is the data plane.
- [docs/questions/0001](../docs/questions/0001-devices-without-nebula.md): devices that cannot run Nebula.

## Acceptance Criteria

- [x] A maintainer can create an authority and a vault in an empty mesh repository.
- [x] Five shares are printed once and never again.
- [x] Any three shares open the vault and two do not.
- [x] `lock` removes the plaintext.
- [x] A certificate for a node matches the address and groups in `mesh.yml`.
- [x] `plan` runs without the authority key, and `apply` reads the key only to sign a certificate that is missing or due for renewal.

## Summary of Changes

The epic delivered five tasks that are complete: [MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md), [MESHY-006](MESHY-006--the-mesh-has-a-certificate-authority.md), [MESHY-007](MESHY-007--a-node-gets-its-certificate.md), [MESHY-038](MESHY-038--the-vault-carries-its-own-shamir-code.md), and [MESHY-044](MESHY-044--the-tool-cache-survives-a-vault-write.md).

Each criterion was checked against the code on `main`, not against a plan:

| Criterion | Evidence |
|---|---|
| an authority and a vault in an empty repository | `cmd/meshy/ca_test.go` and `ca_integration_test.go` |
| five shares, printed once | `TestCAInitPrintsTheSharesOnce`, `Seal` returns `ShareCount` |
| three shares open, two do not | `internal/vault/vault_test.go:41`, `Open` refuses fewer than `Threshold` |
| `lock` removes the plaintext | `TestLockRemovesThePlaintext`, `TestLockRemovesTheToolCache` |
| the certificate matches the address and the groups | `internal/issue/issue_test.go`, `TestIssueSignsWithTheRealTool` |
| `plan` needs no key, `apply` needs it only to sign | `planInput` reads no key, and `applyRun.certificate` reads one only when `due` is true |

Two notes in this ticket described a design that the project refused. The certificate package is not imported, and the pinned tool does the signing, which [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) decided. The last criterion said that `apply` needs no authority key. That changed when [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md) made `apply` renew a certificate inside the window, so the criterion now says what the code does. A closed epic must not claim a design that is not there.
