---
tags:
  - rotation
  - trust
---

# A rotation reads the signer before it changes the file

In `cmd/meshy/rotate.go`, a phase that drops an authority reads the signing authority from every server node before it edits `mesh.yml`. A node that still holds a certificate from the authority to be dropped stops the phase.

The order is the rule. The file is what the next run reads. A file that drops an authority while a node still signs with it leaves a mesh that cannot repair itself. The node would lose the trust that it needs to reach the rest of the mesh. No later phase could sign for it, because the file no longer holds the authority that signed its certificate.

A node that cannot be read also stops the phase. The run cannot prove what that node holds, and a guess is not proof.

## Enforcement

`TestCARotatePhase3RefusesWhileANodeHoldsTheOldCertificate` asserts three things at once: the phase exits 1, the message names the node and its authority, and `mesh.yml` is byte for byte the same. It also asserts that no node took a write.

`TestCARotatePhase3ChangesOneLineOfMeshYml` asserts that a phase which does pass changes one line and nothing else.

The rotation job asserts the refusal against a real mesh. It runs phase 3 after phase 1, while both containers still hold a ca-1 certificate.

## See also

- [0021](0021-a-rotation-signs-then-reloads.md): a rotation signs then reloads.
- [0020](0020-an-edit-to-mesh-yml-keeps-every-other-byte.md): an edit to mesh.yml keeps every other byte.
- [0015](../adr/0015-the-old-key-outlives-the-rotation.md): the old key outlives the rotation.
