---
tags:
  - vault
  - keys
---

# The vault is a file sealed with age and Shamir shares

The repository must hold the certificate authority key and every node key, because the pipeline issues certificates and regenerates bundles. Those keys live in one `secrets.age` file. The file key is split into five Shamir shares, and any three unseal it. Shares come from the password manager locally and from repository secrets in CI. `meshy unlock` writes plaintext to a temporary directory and `meshy lock` removes it.

## Consequences

A compromised pipeline can decrypt the vault and mint any identity in the mesh. No key split prevents that. The design answer is cheap recovery: rotation and re-issue must be routine commands.

Two commands need the vault open: `issue` and `bundle`. `plan` and `apply` need it only to read node keys when they must rewrite a node config that is missing.
