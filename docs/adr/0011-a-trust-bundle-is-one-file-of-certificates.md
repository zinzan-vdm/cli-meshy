---
tags:
  - nebula
  - trust
  - rotation
---

# A node trusts a bundle of concatenated certificates

Nebula names one file in `pki.ca`. A node that must accept two authorities reads one file. That file holds the certificate of every authority in `ca.trust`, in the order that the mesh file names them. `apply` writes that file, and `meshy ca rotate --phase 1` adds one certificate to it.

## Evidence

`nebula` v1.12.0 was run with a `pki.ca` file that held the certificate of `ca-1` and the certificate of `ca-2`. The debug log named both: `Trusted CA fingerprints [cded78a3 cf7ac451]`. The same binary refused a directory at that path with `input did not contain a valid PEM encoded block`. The file is the only form the pinned release reads.

`nebula-cert print -json` prints one document for each PEM block, so the read back of `ca.crt` returns an array of two. `node.ParseCerts` reads every document, and the plan compares the whole set with `ca.trust`.

## Consequences

A node holds one file and one hash, so a change to the trust list is one write and one reload.

The order of the certificates is the order of `ca.trust`, so the bytes of the bundle are stable for a given mesh file. Nebula does not care about the order.

A node that holds only the old certificate still works, because the bundle that it holds names the authority that signed it. Phase 1 therefore needs no certificate and no restart of a tunnel.

`meshy bundle` still writes one authority for a user node. A user node takes the whole bundle on its next `meshy bundle`.

## See also

- [0001](0001-nebula-is-the-data-plane.md): Nebula is the data plane.
- [0005](0005-the-vault-is-sealed-with-age-and-shares.md): the vault is sealed with age and shares.
