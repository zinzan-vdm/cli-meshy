---
# MESHY-034
title: Every node holds a new certificate
status: in-progress
type: task
priority: high
tags:
    - certificates
    - rotation
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T19:05:00Z
parent: MESHY-024
blocked_by:
    - MESHY-025
---

## Overview

`meshy ca rotate --phase 2` issues a certificate for every node from the active authority, writes it, and reloads. The mesh keeps running through the change, because every node already trusts both authorities.

## Context / Background

Phase 1 added trust and changed no certificate. Phase 2 is the change itself: every node moves to a certificate signed by the new authority, one node at a time.

A node reloads into its new certificate on a live tunnel. That is the whole reason the rotation is three phases instead of one.

Part of [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the trust anchor rotates without downtime.

## Specification

```
meshy ca rotate --phase 2 [--mesh <dir>]
```

| Step | Work |
|---|---|
| 1 | read `ca.active` from `mesh.yml` |
| 2 | issue a certificate for every node from the active authority |
| 3 | write the certificate and the config to each node |
| 4 | reload each unit and confirm the handshake |
| 5 | read back the signing authority from every node |

| Refusal | Message |
|---|---|
| `ca.trust` holds one authority | `run phase 1 first` |
| a node fails its handshake | report the node, stop, leave the rest untouched |

## Technical Notes

- Certificates keep their overlay address, so a reload does not move a node.
- The write is the same path `apply` uses, so a node that already matches is not touched twice.
- A node that fails stops the phase, so a half rotated mesh is visible in the report.
- The old certificate stays in the vault until phase 3.

## Reference Material

- [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the parent epic.
- [MESHY-025](MESHY-025--every-node-trusts-the-next-authority.md): phase 1.
- [MESHY-035](MESHY-035--the-mesh-drops-the-old-authority.md): phase 3.
- [MESHY-015](MESHY-015--renew-a-certificate.md): the renewal that writes a certificate the same way.

## Acceptance Criteria

- [x] Phase 2 issues a certificate for every node from the active authority.
- [ ] Each node receives its certificate and reloads with no dropped tunnel.
- [x] The old certificate stays valid until phase 3.
- [x] Phase 2 refuses to run while `ca.trust` holds one authority.
- [x] A node that fails its handshake stops the phase and is named in the report.

## Summary of Changes

| File | Change |
|---|---|
| `cmd/meshy/rotate.go` | new: `ca rotate --phase 2`. It refuses a trust of one, picks the new authority, changes `ca.active`, signs, writes, reloads, and reads the issuer back. |
| `internal/config/active.go` | new: `SetActive` edits the `ca.active` line and no other byte of `mesh.yml`. |
| `internal/ca/next.go` | new: `NewActive` returns the trusted authority that outranks `ca.active`. |
| `internal/node/read.go` | new: `ReadCert` reads the certificate of one node, with its issuer. |
| `internal/node/state.go` | `Cert.Issuer` holds the fingerprint of the authority that signed a certificate. |
| `internal/vault/node.go` | new: `KeepPrevious`, `Previous`, and `PreviousAuthorities` keep the old pair under `<node>.crt.previous.<authority>`. |
| `internal/apply/apply.go` | new: `Report.Changed` reports whether a run wrote a file. |
| `cmd/meshy/rotate_test.go` | the phase 2 tests, and the phase 3 refusal test. |
| `cmd/meshy/rotate_integration_test.go` | new: the real `nebula-cert` probe of the two chains. |
| `cmd/meshy/apply_test.go` | the fake node answers the read back with the value that a write leaves. |
| `test/e2e/rotate/` | new: the live rotation scenario, its `mesh.yml`, its compose file, its image, and the `systemctl` shim. |
| `.github/workflows/rotate.yml` | new: the one job that runs the live rotation. |
| `docs/adr/0012-phase-2-changes-the-active-authority.md` | new: the decision and the probe. |
| `docs/patterns/0021-a-rotation-signs-then-reloads.md` | new: the order of a rotation. |
| `docs/patterns/TAGS` | the `rotation` tag. |
| `README.md` | the phase 2 section and the command table. |
| `.gitignore` | the build and run directories of the rotation scenario. |

## Verification

`TestCARotatePhase2SignsEveryNodeWithTheNewAuthority` proves that both server nodes take a certificate from `ca-2`, reload once, and that the vault keeps the pair that `ca-1` signed. `TestCARotatePhase2ChangesOneLineOfMeshYml` proves the edit changes one line and no other byte.

`TestCARotatePhase2SignsWithTheRealTool` runs `ca init`, `apply`, phase 1, and phase 2 with `nebula-cert` v1.12.0. The new certificate verifies against `ca-2` and fails against `ca-1`. The kept certificate still verifies against `ca-1`. That is the proof of the first and the third criterion.

`TestCARotatePhase2RefusesWhileTrustHoldsOneAuthority`, `TestCARotatePhase2SaysUnlockWhenTheVaultIsClosed`, `TestCARotatePhase2StopsAtANodeThatFailsAndLeavesTheRest`, `TestCARotatePhase2ReportsANodeThatKeptTheOldCertificate`, `TestCARotatePhase2ReportsNoChangeWhenTheNewAuthorityIsActive`, and `TestCARotatePhase2FinishesANodeThatAStoppedRunLeft` cover the refusals, the stop, the read back, and the second run.

`TestSetActive*`, `TestNewActive*`, `TestReadCert*`, and `TestKeepPrevious*` cover the parts that the command builds on.

The gates on this host: `gofmt -l .` prints nothing. `go build ./...`, `go vet ./...`, and `go test ./...` pass. The run counts 23 packages with zero `--- FAIL`. `beans check` passes, and the write-tism audit prints `total: 0`.

## Technical Notes

The command picks the highest numbered trusted authority, and that name must outrank `ca.active`. With the two names that phase 1 leaves, the new authority is the one that phase 1 created. The decision is in [ADR 0012](../docs/adr/0012-phase-2-changes-the-active-authority.md).

The command reads the issuer of each node before it signs. It does not touch a node that the new authority already signed. A second run therefore finishes a stopped phase, and a complete mesh reports `no change`.

The command reads the certificate back after the reload. The `issuer` field must name the new authority, or the node failed and the phase stops there.

## Deviations

- The criterion "Each node receives its certificate and reloads with no dropped tunnel" is not proved here. This host has no Docker and no `/dev/net/tun`, so no tunnel came up. `.github/workflows/rotate.yml` will prove it. That job runs phase 1 and phase 2 against live containers. It pings across the tunnel at 0.2 s and requires zero lost replies. It also compares the certificate fingerprint before and after. The scenario was not run here, so the criterion stays unticked until that job is green.
- The live scenario answers `systemctl` with a shim. A container has no init, and the shim sends SIGHUP to the real `nebula` process, so a reload is a reload. The real unit path is the subject of MESHY-045.
- The command prints `next: meshy ca rotate --phase 3` only when every node came back. A stopped phase names the node and asks for phase 2 again. Phase 3 would drop the old authority while a node still signs with it.
- A kept pair from an authority that the mesh does not trust takes the issuer fingerprint as its name. The pair must not lose its record.
