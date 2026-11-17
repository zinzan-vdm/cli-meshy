---
tags:
  - mesh
  - repository
---

# The repository is the mesh

One git repository describes one mesh. The repository holds `mesh.yml` and `secrets.age` and nothing else that matters. We ship no multi-mesh mode, so a merge has exactly one meaning: apply this mesh. A host that joins two meshes appears in two repositories, and each repository manages only its own Nebula process on that host.

## Consequences

There is no registry of meshes. Every command takes `--mesh <dir>` and resolves `<dir>/mesh.yml`, defaulting to the current directory.

Each repository carries its own workflow, its own certificate authority, and its own vault.

Withdrawal of the multi-mesh folder idea means a shared host is configured by two independent applies. The two must not write the same config directory or claim the same interface.
