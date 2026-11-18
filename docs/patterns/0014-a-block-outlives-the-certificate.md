---
tags:
  - revocation
  - vault
---

# A block outlives the certificate

In `internal/vault`, keep the fingerprint of a blocked certificate in the record after the certificate and the key leave.

## Rationale

A run blocks a node because the mesh file no longer names it. If the fingerprint left with the certificate, the next run would build a blocklist from the remaining certificates. The blocklist would shrink, and a node that returned with its old certificate would be trusted again.

The fingerprint is the only part of a blocked certificate that a node needs. It is small, it carries no key material, and it keeps the decision after the reason is gone.

## Enforcement

`TestTextKeepsABlockThatTheRecordAlreadyHolds` asserts a blocklist from the record alone. `TestApplyRevokesANodeThatLeftTheMesh` asserts the certificate leaves the vault and the blocklist still holds the fingerprint.

## See also

- [0015](0015-the-blocklist-is-a-file-the-config-names.md): the file that carries the value.
- [0005](0005-the-vault-record-holds-one-file-per-name.md): the record shape.
