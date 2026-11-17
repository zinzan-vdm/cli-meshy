---
id: MESHY-024
title: The trust anchor rotates without downtime
type: epic
status: todo
priority: low
tags: [ca, rotation]
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
- [docs/mesh-yml.md](../docs/mesh-yml.md): the `ca` block.
- [Nebula CA rotation guide](https://nebula.defined.net/docs/guides/rotating-certificate-authority/): the mechanics.

## Acceptance Criteria

- [ ] The plan reports the age of the active authority and the next phase.
- [ ] A mesh can hold two trusted authorities and keep working.
- [ ] Every node holds a certificate from the new authority after phase 2.
- [ ] Phase 3 is refused while a node still carries an old certificate.
- [ ] No phase drops a tunnel that stays within the mesh.
- [ ] A full rotation is tested against two nodes before release.
