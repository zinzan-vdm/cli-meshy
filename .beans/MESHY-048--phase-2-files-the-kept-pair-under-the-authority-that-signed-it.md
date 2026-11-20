---
# MESHY-048
title: Phase 2 files the kept pair under the authority that signed it
status: completed
type: bug
priority: normal
tags:
    - rotation
    - vault
    - defect
created_at: 2026-11-19T10:38:40Z
updated_at: 2026-11-20T19:32:25Z
---

## Overview

Phase 2 keeps the certificate that the vault holds for a node. It stores the pair under `<node>.crt.previous.<authority>`. The authority name comes from the certificate on the node, not from the certificate that is kept. When the two differ, the pair is filed under the wrong name.

## Context / Background

A node that was unreachable during an earlier phase 2 keeps its old certificate. The vault already holds a newer one. The next phase 2 signs from the new authority and replaces the vault pair again. The pair that it keeps was signed by ca-2, but the node still holds a ca-1 certificate, so the label reads `ca-1`.

The trust is correct. Phase 3 reads `ca.trust` and the node, and never the label. Only the kept pair is mislabelled.

The defect was found while writing [MESHY-035](MESHY-035--the-mesh-drops-the-old-authority.md). It was left alone on purpose, because the rotation pull request was already large.

## Specification

`vault.KeepPrevious` receives the authority name. The caller passes the issuer of the certificate that the node holds. It must pass the issuer of the certificate that `vault.Certificate` returns.

| Certificate on the node | Certificate in the vault | Label today | Label after |
|---|---|---|---|
| ca-1 | ca-1 | ca-1 | ca-1 |
| ca-1 | ca-2 | ca-1 | ca-2 |
| ca-2 | ca-2 | ca-2 | ca-2 |

The record shape does not change. The exit codes do not change. A refusal does not change.

When the run cannot read the issuer of the kept certificate, or the issuer is empty, it reports an error for that node. It writes no kept pair and no new certificate, so the node keeps the pair that it has.

## Technical Notes

- `cmd/meshy/rotate.go`: the `vault.KeepPrevious` call in the phase 2 path, next to `signerOf(signers, held.Issuer)`.
- `internal/vault/node.go`: `KeepPrevious`, `Previous`, and `PreviousAuthorities` already carry the authority in the key. No change to the record.
- Read the issuer from the kept certificate bytes with the same reader that phase 2 uses to read `details.issuer` from a node certificate. One code path, so both labels come from a certificate.
- A CA certificate holds no issuer. Never file a pair under an empty name. That case is the error above.
- `cmd/meshy/rotate_integration_test.go` holds a test that reads a kept pair. Add a case where the node is behind: the vault holds a ca-2 pair and the node holds a ca-1 pair.
- `signerOf` may become unused. Remove it only when nothing else calls it.

## Reference Material

- [MESHY-034](MESHY-034--every-node-holds-a-new-certificate.md): phase 2, which writes the kept pair.
- [MESHY-035](MESHY-035--the-mesh-drops-the-old-authority.md): phase 3, which reads the authorities of the kept pairs.
- [0012](../docs/adr/0012-phase-2-changes-the-active-authority.md): phase 2 keeps the replaced pair under the authority that signed it.
- [0015](../docs/adr/0015-the-old-key-outlives-the-rotation.md): the kept pair stays after phase 3.
- [0021](../docs/patterns/0021-a-rotation-signs-then-reloads.md): a rotation signs, then reloads.

## Acceptance Criteria

- [x] The kept pair of a node that is behind is stored under the authority that signed the kept certificate.
- [x] The kept pair of a node that is current is stored under the same name as before.
- [x] A test covers a node that is behind, and it fails when the label comes from the node certificate.
- [x] Phase 2 refuses the node and writes no kept pair when the kept issuer is empty or unreadable.
- [x] The node keeps its current certificate when phase 2 refuses it.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass.
- [x] `beans check` passes.

## Summary of Changes

Phase 2 files the kept pair under the authority that signed it.

### Files

- `cmd/meshy/rotate.go`: phase 2 reads the issuer of the kept certificate with `issue.Print` and files the pair under that name. An empty issuer stops the node before any write.
- `cmd/meshy/rotate_integration_test.go`: a subtest where the vault holds a ca-2 pair and the node holds a ca-1 certificate.
- `cmd/meshy/rotate_test.go`: a test where the kept certificate names no issuer, so phase 2 refuses the node.
- `cmd/meshy/issue_test.go`: the stub print names an issuer, because a rotation reads the field.

### Decisions

- The label comes from `issue.Print` on the kept certificate, which is the reader that phase 2 uses for `details.issuer`. One code path, so both labels come from a certificate.
- The issuer is read before `issue.Sign`, so a refusal writes no kept pair, no vault certificate, and no node file.
- `signerOf` stays, because phase 3 names the signer in its refusal message.

### Deviations

None.

### Measurements

- `gofmt -l .` silent, `go build ./...`, `go vet ./...`, and `go test ./...` pass.
- `TestCARotatePhase2SignsWithTheRealTool/a_node_that_is_behind_keeps_its_pair_under_its_signer` passes with `nebula-cert` 1.12.0. It fails when the label comes from the node certificate, with `the vault filed the kept pair under [ca-1], want ca-2`.
- `TestCARotatePhase2RefusesAKeptCertificateWithNoIssuer` passes.
- The live rotation job did not run on this host, which has no Docker and no `/dev/net/tun`.
