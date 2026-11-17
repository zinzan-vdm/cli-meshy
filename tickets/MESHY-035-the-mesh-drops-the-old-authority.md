---
id: MESHY-035
title: The mesh drops the old authority
type: task
status: todo
priority: normal
parent: MESHY-024
depends_on:
  - MESHY-034
tags: [certificates, rotation]
---

## Overview

`meshy ca rotate --phase 3` removes the old authority from `ca.trust` and applies the new bundle to every node. The rotation is finished, and the old key stays in the vault for the record.

## Context / Background

Trusting an authority nobody uses is a standing risk. Phase 3 closes it, and it refuses to run while any node still holds a certificate from the old authority.

The key is kept, not deleted. A deleted key cannot explain a certificate that is still in a backup.

## Specification

```
meshy ca rotate --phase 3 [--mesh <dir>]
```

| Step | Work |
|---|---|
| 1 | read the signing authority from every node |
| 2 | remove the old authority from `ca.trust` in `mesh.yml` |
| 3 | apply the new bundle to every node |
| 4 | read back the trust bundle from every node |

| Refusal | Message |
|---|---|
| a node still holds an old certificate | `web-1 still holds a certificate from ca-1` |
| `ca.trust` holds one authority | `the rotation is already complete` |

## Technical Notes

- Phase 3 never deletes `ca-1.key` from the vault. It only leaves `ca.trust`.
- The read back is the proof that no node can still accept the old authority.
- A rotation against two nodes completes with no dropped tunnel.

## Reference Material

- [MESHY-024](MESHY-024-the-trust-anchor-rotates.md): the parent epic.
- [MESHY-034](MESHY-034-every-node-holds-a-new-certificate.md): phase 2.
- [MESHY-016](MESHY-016-remove-a-node-from-the-mesh.md): the other command that pushes a blocklist.

## Acceptance Criteria

- [ ] Phase 3 removes the old authority from `ca.trust` and applies the new bundle.
- [ ] The old key stays in the vault.
- [ ] Phase 3 is refused while a node still holds a certificate from the old authority.
- [ ] The mesh completes the rotation with no dropped tunnel.
- [ ] The command reports that the rotation is complete.
