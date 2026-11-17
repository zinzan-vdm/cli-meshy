---
tags:
  - addressing
  - state
---

# Overlay addresses are explicit and the nodes hold the state

Every node carries an explicit `ip` in `mesh.yml`. cli-meshy keeps no address pool and no allocation file. `plan` learns the current state by reading each server node: certificate, interface, service, and binary version. For user nodes, which cannot be read, the record in the vault is the state.

## Consequences

State cannot diverge between the repository, a state file, and the nodes, because there is no state file.

Renaming a node keeps its address only because the address is written down. Moving a node to a new address is an edit in a reviewed pull request.

A user node that disappears locally stays in the vault until its record is removed. Removal is an explicit `apply` step, not a discovery step.
