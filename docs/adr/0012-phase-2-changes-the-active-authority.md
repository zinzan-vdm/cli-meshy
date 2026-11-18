---
tags:
  - certificates
  - rotation
  - trust
---

# Phase 2 changes the active authority and signs again

`meshy ca rotate --phase 2` makes the new authority active and replaces the certificate of every server node. Every node already trusts both authorities, so a node reloads into its new certificate without a restart.

Phase 1 added the trust. Phase 2 is the change itself. Phase 3 removes the old authority.

## The decision

1. `ca.active` names the authority that signs new certificates. Phase 2 changes that name in `mesh.yml` with one line edit. See [0020](../patterns/0020-an-edit-to-mesh-yml-keeps-every-other-byte.md).
2. The new authority is the highest numbered authority in `ca.trust`, and it must outrank `ca.active`. An active authority that already holds the highest number means the mesh file names the new authority. The command then reads the issuer of every node and touches only a node that another authority signed. A mesh that the new authority already signs is reported as no change.
3. Every server node gets a certificate from the new authority. The certificate keeps the address and the groups, so the node needs a reload and not a restart.
4. The vault keeps the pair that the rotation replaced, under `<node>.crt.previous.<authority>`. That suffix does not end in `.crt`, so the revocation never reads a kept pair as a node.
5. The phase stops at the first node that does not come back. The command names that node and every node that it did not reach, so a half rotated mesh is visible in the report.
6. The command does not print the next phase after a stop. Phase 3 would drop the old authority while a node still signs with it.

## Evidence

`nebula-cert` v1.12.0 was run against a real rotation on a host with no container runtime. The new certificate verifies against `ca-2` and fails against `ca-1`. The kept certificate still verifies against `ca-1`, so the old certificate stays valid until phase 3. `TestCARotatePhase2SignsWithTheRealTool` repeats that probe.

`nebula-cert print -json` names the signing authority in `details.issuer`. The command reads that field from each node, so the report names the authority that signed the certificate that the node holds.

## Consequences

A node that the run does not reach keeps its old certificate and keeps working. The mesh file names the new authority before the first node is touched. A second `ca rotate --phase 2` therefore finishes the nodes that the first run did not reach.

A rotation doubles the certificates in the vault for the length of one rotation. Phase 3 removes the old authority and the pairs that it signed.

## See also

- [0011](0011-a-trust-bundle-is-one-file-of-certificates.md): a trust bundle is one file of certificates.
- [0008](0008-the-nebula-tools-do-the-signing.md): the Nebula tools do the signing.
