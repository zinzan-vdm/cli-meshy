---
id: MESHY-025
title: Every node trusts the next authority
type: task
status: todo
priority: high
parent: MESHY-024
depends_on:
  - MESHY-006
tags: [certificates, rotation]
---

## Overview

`meshy ca rotate --phase 1` creates the next authority, adds it to `ca.trust`, and applies the new trust bundle to every node. The mesh keeps running, and every node still holds a certificate from the old authority.

## Context / Background

The authority key is the root of trust, and a mistake here locks every node out. Phase 1 is the safe half of the rotation: it adds trust without changing any certificate.

Every later phase refuses to run until phase 1 is done, so a maintainer cannot start in the middle.

## Specification

```
meshy ca rotate --phase 1 [--mesh <dir>]
```

| Step | Work |
|---|---|
| 1 | create `ca-2`, sealed into the vault |
| 2 | add `ca-2` to `ca.trust` in `mesh.yml` |
| 3 | apply the new trust bundle to every node |
| 4 | read back the authority list from every node |

| Refusal | Message |
|---|---|
| `ca.trust` already holds two authorities | `ca.trust already holds two authorities` |
| the vault is locked | `the vault is locked, run meshy unlock` |

The command prints the phase that comes next.

## Technical Notes

- The command edits `mesh.yml`, so the rotation produces a diff a reviewer can read.
- Phase 1 issues no certificate, so a node that is not reached keeps working.
- The read back at the end is the proof that every node took the new bundle.
- The next authority is named `ca-<n+1>`, so a rotation reads the same every time.

## Reference Material

- [MESHY-024](MESHY-024-the-trust-anchor-rotates.md): the parent epic.
- [MESHY-034](MESHY-034-every-node-holds-a-new-certificate.md): phase 2.
- [MESHY-006](MESHY-006-the-mesh-has-a-certificate-authority.md): the authority creation that phase 1 reuses.

## Acceptance Criteria

- [ ] Phase 1 creates an authority and adds it to `ca.trust`.
- [ ] The new trust bundle reaches every node.
- [ ] The mesh keeps working with every node on the old certificate.
- [ ] Phase 1 refuses to run when `ca.trust` already holds two authorities.
- [ ] The command prints the phase that comes next.
