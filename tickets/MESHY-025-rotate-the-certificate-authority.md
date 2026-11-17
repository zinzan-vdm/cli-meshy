---
id: MESHY-025
title: Rotate the certificate authority
type: task
status: todo
priority: low
parent: MESHY-024
tags: [ca, rotation]
---

## Overview

`meshy ca rotate` walks the three phases of a rotation and refuses any step whose precondition is not met. The maintainer makes the change in `mesh.yml`, and the command does the certificate work.

## Context / Background

The authority key is the root of trust, and a mistake here locks every node out. The command exists to make the sequence mechanical and to stop a maintainer from skipping phase 1.

## Specification

```
meshy ca rotate --phase 1 [--mesh <dir>]
meshy ca rotate --phase 2
meshy ca rotate --phase 3
```

| Phase | Precondition | Work |
|---|---|---|
| 1 | `ca.trust` holds one authority | create the next authority, add it to `ca.trust`, apply the new trust bundle to every node |
| 2 | `ca.trust` holds two, `ca.active` is the new one | issue a certificate for every node from the active authority, write, reload |
| 3 | every node carries a certificate from the active authority | remove the old authority from `ca.trust`, apply, keep the key in the vault |

| Refusal | Message |
|---|---|
| phase 1 with two authorities | `ca.trust already holds two authorities` |
| phase 2 with one authority | `run phase 1 first` |
| phase 3 with a node on the old authority | `web-1 still holds a certificate from ca-1` |

## Technical Notes

- The command edits `mesh.yml`, so a rotation produces a diff a reviewer can read.
- Phase 3 never deletes `ca-1.key` from the vault. It only leaves `ca.trust`.
- Every phase ends with a read back of every node's authority list.

## Reference Material

- [MESHY-024](MESHY-024-the-trust-anchor-rotates.md): the parent epic.
- [MESHY-006](MESHY-006-create-the-certificate-authority.md): the authority creation that phase 1 reuses.

## Acceptance Criteria

- [ ] Phase 1 creates an authority and adds it to `ca.trust`.
- [ ] Phase 1 leaves the mesh working with nodes on the old authority.
- [ ] Phase 2 reissues every certificate from the active authority.
- [ ] Phase 3 is refused while a node still holds an old certificate.
- [ ] Phase 3 leaves the old key in the vault.
- [ ] Each phase prints the phase that comes next.
- [ ] A rotation against two nodes completes with no dropped tunnel.
