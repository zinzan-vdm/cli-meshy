---
# MESHY-015
title: Renew a certificate inside the window
status: completed
type: task
priority: normal
tags:
    - certificates
    - apply
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T09:30:00Z
parent: MESHY-013
---
## Overview

`apply` renews any certificate that is inside `certificates.renew_before` of expiry, writes it to the node, and reloads. Renewal keeps the same address and the same groups, so no peer notices.

## Context / Background

Certificates last one year by default, and a repository may go months without a merge. Renewal on every apply, plus a weekly scheduled check, is the mechanism that keeps the mesh alive without a calendar reminder.

Part of [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): a merge changes the mesh.

## Specification

| Condition | Action |
|---|---|
| certificate missing on the node | issue, write, reload |
| remaining life at or below `renew_before` | issue, write, reload |
| certificate in the vault differs from the node | write the vault copy, reload |
| remaining life above the window | nothing |

Renewal uses the authority named by `ca.active`. A user node is never renewed remotely. Its bundle is rebuilt and the maintainer hands it over.

## Technical Notes

- Issuance is the same code path as `meshy issue`, so the two cannot drift.
- The new certificate keeps the address, which is what allows a reload instead of a restart.
- A renewal inside a run that also changes rules produces one reload, not two.

## Reference Material

- [MESHY-007](MESHY-007--a-node-gets-its-certificate.md): the issuance path.
- [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): the parent epic.
- [docs/conf/0001-mesh-yml.md](../docs/conf/0001-mesh-yml.md): the `certificates` block.

## Acceptance Criteria

- [x] A certificate inside the window is replaced and the node reloads.
- [x] A certificate outside the window is left alone.
- [x] The renewed certificate keeps the address and the groups.
- [x] A user node is never renewed over SSH.
- [x] A rule change and a renewal in one run cause one reload.
- [x] The plan reports the same renewal before it happens.

## Summary of Changes

| File | Change |
|---|---|
| `cmd/meshy/apply.go` | `applyRun.certificate` signs a certificate when the vault holds none, or when the one it holds is inside `certificates.renew_before`. `due` reads the remaining life with the pinned tool. |
| `cmd/meshy/issue.go` | `meshNetworks` and `lifetime` are shared with `meshy issue`, so the two cannot drift. |
| `internal/apply/apply.go` | `Files` carries the blocklist. `Unit` is exported for a caller that checks the unit before it signs. |
| `internal/revoke` | The package that finds the certificates the mesh no longer names. |
| `docs/patterns/0016` | A renewal signs again from the mesh file. |
| `README.md` | The apply section names the renewal and the exit contract. |

## Deviations

The run reads the remaining life from the certificate in the vault, not from the certificate on the node. The vault copy is the certificate the run writes. A node that holds no file, or an older one, receives the vault copy through the write path. The first row of the table is the same code, and the run saves a read of every node.

## Verification

`TestApplyRenewsACertificateInsideTheWindow`, `TestApplyLeavesACertificateOutsideTheWindow`, and `TestApplyRenewsAndRevokesWithTheRealTool` cover the three cases. The last one runs only when `MESHY_NEBULA_CERT` names a binary, and it signed a certificate of one hour with the real tool.
