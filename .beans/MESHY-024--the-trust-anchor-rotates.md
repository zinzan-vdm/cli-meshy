---
# MESHY-024
title: The trust anchor rotates without downtime
status: completed
type: epic
priority: low
tags:
    - ca
    - rotation
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T23:55:00Z
---
## Overview

A maintainer can replace the certificate authority while the mesh keeps running. The rotation is three reviewed changes, and each one leaves the mesh working.

## Context / Background

The default authority lasts twenty years, so rotation is rare. Rare and unrehearsed is the dangerous combination. The tool must make the three phases mechanical, because the failure mode is a mesh that trusts nobody.

## Specification

Three changes, in order:

| Phase | Change to `mesh.yml` | Effect |
|---|---|---|
| 1 | add `ca-2` to `ca.trust` | every node trusts both authorities |
| 2 | set `ca.active: ca-2`, then `meshy issue --all --force` | every node holds a certificate from `ca-2` |
| 3 | remove `ca-1` from `ca.trust` | only the new authority is trusted |

`plan` reports the age of the active authority against `ca.rotate_at` and prints the phase that comes next.

## Technical Notes

- Phase 1 is safe on its own. A node that has not been reached yet still trusts `ca-1`.
- Phase 3 is the only irreversible step, and the plan refuses it while any node still carries an old certificate.
- Rotation never deletes the old authority from the vault. It stays as a record.

## Reference Material

- [ADR 0001](../docs/adr/0001-nebula-is-the-data-plane.md): the trust model.
- [docs/conf/0001-mesh-yml.md](../docs/conf/0001-mesh-yml.md): the `ca` block.
- [Nebula CA rotation guide](https://nebula.defined.net/docs/guides/rotating-certificate-authority/): the mechanics.

## Acceptance Criteria

- [x] The plan reports the age of the active authority and the next phase.
- [x] A mesh can hold two trusted authorities and keep working.
- [x] Every node holds a certificate from the new authority after phase 2.
- [x] Phase 3 is refused while a node still carries an old certificate.
- [x] No phase drops a tunnel that stays within the mesh.
- [x] A full rotation is tested against two nodes before release.

## Status

All six criteria are proved. Two of them come from a plan run, and four from live runs against two containers.

| Criterion | Evidence |
|---|---|
| The plan reports the age of the active authority and the next phase | MESHY-046, merged. `meshy plan` prints the block, and `TestPlanPrintsTheAgeOfTheAuthority` runs the command against a test node. |
| A mesh holds two trusted authorities and keeps working | run `44423734094` line 4, then line 7: 0 of 9 replies lost while both authorities were trusted. |
| Every node holds a certificate from the new authority after phase 2 | run `44423734094` lines 6 and 8, with the fingerprints read back. |
| Phase 3 is refused while a node carries an old certificate | run `44423734094` line 9. |
| No phase drops a tunnel that stays within the mesh | run `44423734094` line 7 for phase 2, line 10 for phase 3, 0 of 9 and 0 of 10 replies lost. |
| A full rotation is tested against two nodes before release | the `live rotation` job, which builds two containers, a vault, and a tunnel in the run. |

A rotation is three commands. Phase 1 adds the trust, phase 2 changes the active authority and signs again, and phase 3 drops the old authority. The old key stays in the vault, and ADR 0015 records why.
