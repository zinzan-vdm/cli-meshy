---
tags:
  - apply
  - certificates
---

# A renewal signs again from the mesh file

In `cmd/meshy`, renew a certificate by signing a new one from the values in `mesh.yml`. Never copy the facts of the certificate that it replaces.

## Rationale

A renewal must keep the address and the groups, because a peer that already knows the node only survives a reload. Copying the old certificate is the short path, and it carries every fact that the mesh file may have changed since. A node whose groups moved would keep the old groups until its certificate expired.

The signing request already reads the name, the address, the groups, and the lifetime from the mesh file. A renewal is the same request, made again.

## Enforcement

`TestApplyRenewsACertificateInsideTheWindow` asserts a renewal inside the window. `TestApplyRenewsAndRevokesWithTheRealTool` signs a certificate with the real tool and asserts the run replaces it. `TestApplyLeavesACertificateOutsideTheWindow` asserts no renewal outside the window.

## See also

- [0014](0014-a-block-outlives-the-certificate.md): the other end of a certificate lifetime.
- [0010](0010-the-unit-reloads-once-after-the-last-write.md): one refresh after the writes.
