---
id: MESHY-034
title: Every node holds a new certificate
type: task
status: todo
priority: high
parent: MESHY-024
depends_on:
  - MESHY-025
tags: [certificates, rotation]
---

## Overview

`meshy ca rotate --phase 2` issues a certificate for every node from the active authority, writes it, and reloads. The mesh keeps running through the change, because every node already trusts both authorities.

## Context / Background

Phase 1 added trust and changed no certificate. Phase 2 is the change itself: every node moves to a certificate signed by the new authority, one node at a time.

A node reloads into its new certificate on a live tunnel. That is the whole reason the rotation is three phases instead of one.

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

- [MESHY-024](MESHY-024-the-trust-anchor-rotates.md): the parent epic.
- [MESHY-025](MESHY-025-every-node-trusts-the-next-authority.md): phase 1.
- [MESHY-035](MESHY-035-the-mesh-drops-the-old-authority.md): phase 3.
- [MESHY-015](MESHY-015-renew-a-certificate.md): the renewal that writes a certificate the same way.

## Acceptance Criteria

- [ ] Phase 2 issues a certificate for every node from the active authority.
- [ ] Each node receives its certificate and reloads with no dropped tunnel.
- [ ] The old certificate stays valid until phase 3.
- [ ] Phase 2 refuses to run while `ca.trust` holds one authority.
- [ ] A node that fails its handshake stops the phase and is named in the report.
