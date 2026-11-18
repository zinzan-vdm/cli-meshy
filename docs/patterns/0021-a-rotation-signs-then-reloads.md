---
tags:
  - certificates
  - rotation
---

# A rotation signs, seals, writes, and then reloads

A command that replaces a node certificate works in one order. It signs from the new authority, seals the pair in the vault, writes the node, reloads the unit once, and reads the issuer back.

The vault comes before the node. A node that holds a certificate that the vault does not hold cannot be repaired by the next run.

The reload comes after the last write, so one change causes one reload. See [0010](0010-the-unit-reloads-once-after-the-last-write.md).

The read back is the proof. A write can report success and still leave the old file. The command reads the certificate on the node and compares its issuer with the new authority.

A node that fails stops the phase. The command leaves the rest untouched, because a node that is not reached still works on its old certificate.

## Enforcement

`TestCARotatePhase2SignsEveryNodeWithTheNewAuthority` asserts one reload for each node and the old pair in the vault.

`TestCARotatePhase2ReportsANodeThatKeptTheOldCertificate` asserts that a stale issuer fails the phase.

`TestCARotatePhase2StopsAtANodeThatFailsAndLeavesTheRest` asserts that the node after a failure sees no command.

## See also

- [0020](0020-an-edit-to-mesh-yml-keeps-every-other-byte.md): an edit to mesh.yml keeps every other byte.
- [0009](0009-a-write-to-a-node-replaces-the-file-by-rename.md): a write to a node replaces the file by rename.
