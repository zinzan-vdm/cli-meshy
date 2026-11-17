---
# MESHY-006
title: The mesh has a certificate authority
status: completed
type: task
priority: high
tags:
    - ca
    - keys
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T23:20:00Z
parent: MESHY-004
---
## Overview

`meshy ca init` creates the first certificate authority for a mesh, seals it into the vault with five shares, and prints those shares once. After this task the mesh has a trust anchor that every node will carry.

## Context / Background

The authority is the root of trust for the mesh, and it must outlive the nodes. The design sets a twenty year default so that rotation is a planned event rather than an emergency. The key never leaves the vault and is never written to a node.

Part of [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the vault seals and unseals the mesh keys.

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

- `nebula-cert ca` creates the authority. The binary is found in `.meshy/bin/<version>/<os>-<arch>/` first and on `PATH` second, per ADR 0008.
- `mesh.yml` gains `ca.trust: [ca-1]` and `ca.active: ca-1` if the keys are missing.
- The vault record holds `ca-1.key`, `ca-1.crt`, and the issue time, so `plan` can report the age.
- A test signs a host certificate with the created authority and checks the issuer fingerprint. It runs when `MESHY_NEBULA_CERT` names a real binary, so the default run needs no download.

## Reference Material

- [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the parent epic.
- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault design.
- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md): why the binary signs and the code does not import Nebula.
- [docs/conf/0001-mesh-yml.md](../docs/conf/0001-mesh-yml.md): the `ca` block.

## Acceptance Criteria

- [x] `meshy ca init` writes `secrets.age` and prints five shares.
- [x] The authority lifetime matches `ca.duration`.
- [x] The authority address range matches `cidr`.
- [x] A second run without `--force` refuses and changes nothing.
- [x] The vault opens with three shares and holds the authority key and certificate.
- [x] A host certificate signed by the new authority names the authority as issuer.
- [x] The shares are printed once and never written to a file.

## Summary of Changes

`meshy ca init` creates `ca-1`, seals its key and certificate into `secrets.age`, prints five shares once, and adds the authority to `mesh.yml` when the file does not name it.

### Files

- `internal/ca/ca.go`: `Init` runs the tool and returns the key, the certificate, and the issue time. `Find` looks in the release cache and then on `PATH`.
- `internal/ca/ca_test.go`: the cache, the fallback, the error that names both places, and the lifetime in hours.
- `internal/config/authority.go`: `SetAuthority` adds `ca.trust` and `ca.active` through a YAML node, so a file that already names them is not rewritten and a comment above the block survives.
- `internal/config/authority_test.go`: the new block, the gap in an existing block, a complete file left alone, and a comment kept.
- `cmd/meshy/ca.go`: `cmdCA` and `cmdCAInit`, with `--force` and `--mesh`.
- `cmd/meshy/ca_test.go`: eight tests against a stub binary that records its arguments.
- `cmd/meshy/ca_integration_test.go`: the chain proof, run when `MESHY_NEBULA_CERT` names a real binary.
- `cmd/meshy/main.go`: the `ca` dispatch and one usage line.
- `internal/node/state.go`: `ParseCert` reads the array that `nebula-cert` prints.
- `docs/adr/0008-the-nebula-tools-do-the-signing.md`, `docs/adr/TAGS`: the decision and its tag.
- `docs/conf/0001-mesh-yml.md`: `ca.trust` and `ca.active` say that `ca init` writes them.

### Decisions

- The authority is created by the pinned binary, not by an imported package. The module raises the Go floor to 1.26 and pulls gvisor, protobuf, and the Windows WireGuard driver. The tarball already carries the tool.
- The lifetime goes to the tool in hours. A year is 365 days here, so twenty years is `175200h`.
- `mesh.yml` is rewritten only when a key is missing. A complete file keeps its bytes, and a comment above the `ca` block survives the edit.
- The vault record holds `ca-1.key`, `ca-1.crt`, and `ca-1.issued`, so `plan` can report the age of the authority without the key.

### Deviations

- `internal/node.ParseCert` expected a JSON object. `nebula-cert` v1.12.0 prints an array of one, so a plan against a real node would have called every certificate unreadable. The parser now reads both shapes, and a test holds the array.
- `nebula-cert sign` renamed `-ip` to `-networks`, and v1.12.0 requires the new name. The integration test uses `-networks`. MESHY-007 needs the same flag.
