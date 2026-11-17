---
tags:
  - mesh
  - automation
---

# GitHub Actions is the primary executor

The control plane is a git repository plus a workflow, not a server. A pull request runs `meshy plan` and shows the diff. A merge to `main` runs `meshy apply` and writes the nodes. `meshy` runs locally as an escape hatch with the same code path. The runner decrypts the vault with shares from repository secrets, applies, and exits, so no plaintext outlives the job.

## Consequences

The runner needs SSH access to every server node. The key and the addresses live in repository secrets. A bastion is deferred.

A compromised workflow can reach every node. The review gate on `main` and pinned action versions are the controls. Certificate issuance stays behind a separate command so that the frequent path never holds the CA.
