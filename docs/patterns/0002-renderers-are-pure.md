---
tags:
  - nebula
  - rendering
---

# Renderers take values and return bytes

In `internal/nebula`, a renderer takes the mesh, one node, and its certificate record as arguments, and returns bytes. It does not read a file, open a connection, or read the clock.

## Rationale

A pure renderer is testable with a string comparison, and the same bytes can be compared against a node to compute a plan. A renderer that reads a file or a node cannot be tested without one.

## Examples

Good: `Render(mesh, node, record, now) ([]byte, error)`.

Bad: a renderer that reads `/etc/nebula/config.yml` to fill in a missing value.

## See also

- [0001](0001-tests-need-no-root-or-network.md): tests need no root and no network.
