---
tags:
  - rotation
  - keys
  - vault
---

# The old key outlives the rotation

Phase 3 drops the old authority from `ca.trust` and leaves `ca-1.key` in the vault. The key is kept, not deleted, because a deleted key cannot explain a certificate that survives in a backup.

A rotation is the moment when the old authority stops being trusted. Deleting the key at the same moment looks tidy, because nothing signs with it again. The tidy version fails on one case. A node backup or an archive can still hold a certificate that ca-1 signed. A person may need to know whether that certificate was real. Without the key nobody can check the signature. Without the certificate the key is useless, so the pair stays together.

## Consequences

`ca rotate --phase 3` removes one name from `ca.trust` and changes no certificate. The kept certificate pairs that phase 2 wrote as `<node>.crt.previous.<authority>` stay as well, for the same reason.

The vault holds one authority that nothing uses. That is the cost of a record that can still answer a question about the past.

## Considered options

Deleting the key with the trust entry was rejected. It saves no risk, because the key alone cannot sign anything that a node accepts. It does remove the only way to check a certificate that a backup still holds.

## See also

- [0012](0012-phase-2-changes-the-active-authority.md): phase 2 changes the active authority and signs again.
- [0011](0011-a-trust-bundle-is-one-file-of-certificates.md): a trust bundle is one file of certificates.
- [0024](../patterns/0024-a-rotation-reads-the-signer-first.md): a rotation reads the signer first.
