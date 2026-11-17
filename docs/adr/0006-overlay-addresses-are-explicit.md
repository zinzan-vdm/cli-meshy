---
tags:
  - address
  - mesh
---

# Overlay addresses are explicit

Every node carries an explicit `ip` in `mesh.yml`. cli-meshy keeps no address pool and no allocation file, because the address must be readable in the pull request that changes it.

## Consequences

Renaming a node keeps its address only because the address is written down.

Moving a node to a new address is an edit in a reviewed pull request.

`validate` checks the address against `cidr`, and rejects the network and broadcast addresses.
