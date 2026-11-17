---
# MESHY-004
title: The vault seals and unseals the mesh keys
status: todo
type: epic
priority: high
tags:
    - vault
    - keys
    - ca
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

The mesh keys live in the repository and never in plaintext. This epic delivers the vault, the certificate authority, and certificate issuance. When it is complete, a maintainer can seal a vault, open it, and mint a certificate for a node.

## Context / Background

The pipeline issues certificates and regenerates bundles, so it needs the authority key and the node keys. Those keys must travel with the repository, which means they must be encrypted at rest. Five shares seal the vault and any three open it, so no single secret holds the mesh.

Issuance is separated from application. `apply` changes rules and needs no authority key. `issue` mints identity and needs it.

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
- Certificate work imports `github.com/slackhq/nebula/cert`. That module requires Go 1.27.3, so `go.mod` moves when this epic starts.
- No command shells out to `nebula-cert`.

## Reference Material

- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault is sealed with age and Shamir shares.
- [ADR 0001](../docs/adr/0001-nebula-is-the-data-plane.md): Nebula is the data plane.
- [docs/questions/0001](../docs/questions/0001-devices-without-nebula.md): devices that cannot run Nebula.

## Acceptance Criteria

- [ ] A maintainer can create an authority and a vault in an empty mesh repository.
- [ ] Five shares are printed once and never again.
- [ ] Any three shares open the vault and two do not.
- [ ] `lock` removes the plaintext.
- [ ] A certificate for a node matches the address and groups in `mesh.yml`.
- [ ] No command requires the authority key to run `plan` or `apply`.
