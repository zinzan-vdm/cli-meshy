---
tags:
  - plan
  - vault
---

# The nodes hold the state

`plan` learns the current state by reading each server node: its certificate, interface, service, and binary version. For a user node, which cannot be read, the record in the vault is the state. There is no state file.

## Consequences

State cannot diverge between the repository, a state file, and the nodes, because there is no state file.

A user node that disappears locally stays in the vault until its record is removed. Removal is an explicit `apply` step, not a discovery step.
