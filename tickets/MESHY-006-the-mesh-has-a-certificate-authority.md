---
id: MESHY-006
title: The mesh has a certificate authority
type: task
status: todo
priority: high
parent: MESHY-004
tags: [ca, keys]
---

## Overview

`meshy ca init` creates the first certificate authority for a mesh, seals it into the vault with five shares, and prints those shares once. After this task the mesh has a trust anchor that every node will carry.

## Context / Background

The authority is the root of trust for the mesh, and it must outlive the nodes. The design sets a twenty year default so that rotation is a planned event rather than an emergency. The key never leaves the vault and is never written to a node.

## Specification

```
meshy ca init [--mesh <dir>]
```

| Output | Value |
|---|---|
| authority name | `ca-1` |
| address range | the `cidr` from `mesh.yml` |
| lifetime | `ca.duration`, twenty years by default |
| vault | `secrets.age`, created or replaced |
| shares | five, printed once, three needed |

The command refuses when `mesh.yml` fails validation, and refuses when `secrets.age` already exists unless `--force` is given.

The printed shares are the only copy. The command says so and does not write them to disk.

## Technical Notes

- `cert.TBSCertificate.Sign` from `github.com/slackhq/nebula/cert` creates the authority in process.
- `mesh.yml` gains `ca.trust: [ca-1]` and `ca.active: ca-1` if the keys are missing.
- The vault record holds `ca-1.key`, `ca-1.crt`, and the issue time, so `plan` can report the age.
- A test signs a host certificate with the created authority and checks the issuer fingerprint.

## Reference Material

- [MESHY-004](MESHY-004-the-vault-seals-and-unseals.md): the parent epic.
- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault design.
- [docs/mesh-yml.md](../docs/mesh-yml.md): the `ca` block.

## Acceptance Criteria

- [ ] `meshy ca init` writes `secrets.age` and prints five shares.
- [ ] The authority lifetime matches `ca.duration`.
- [ ] The authority address range matches `cidr`.
- [ ] A second run without `--force` refuses and changes nothing.
- [ ] The vault opens with three shares and holds the authority key and certificate.
- [ ] A host certificate signed by the new authority names the authority as issuer.
- [ ] The shares are printed once and never written to a file.
