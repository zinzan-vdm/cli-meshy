---
# MESHY-008
title: A maintainer can see what a merge will change
status: todo
type: epic
priority: high
tags:
    - plan
    - cli
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

`meshy plan` reads the mesh file, reads the nodes, and prints every difference. A pull request runs it, so a reviewer sees the effect of the change before the merge. This epic delivers config rendering, rule expansion, live reads over SSH, and the plan report.

## Context / Background

A mesh file is a description, not the network. The gap between the two is where outages live. A node that runs an old binary, a certificate three days from expiry, and a rule that never reached its destination all hide there. `plan` closes that gap without writing anything.

## Specification

```
meshy plan [--json] [--mesh <dir>]
```

| Result | Exit code |
|---|---|
| no differences | 0 |
| differences | 1 |
| validation problems | 1, and no node is read |

The plan holds four kinds of difference: certificate, config, binary, and rule. Each names the node and the field.

## Technical Notes

- Rendering is a pure function of `mesh.yml`, the vault record, and the node kind, so it is testable without a network.
- Live reads run in parallel across nodes, with a bound on concurrency.
- The plan is computed, never cached. There is no state file.

## Reference Material

- [ADR 0006](../docs/adr/0006-overlay-addresses-are-explicit.md): the nodes hold the state.
- [ADR 0002](../docs/adr/0002-github-actions-is-the-primary-executor.md): CI is the executor.
- [docs/conf/0001-mesh-yml.md](../docs/conf/0001-mesh-yml.md): the fields the plan compares.

## Acceptance Criteria

- [ ] A mesh with no differences exits 0 and prints one line per node.
- [ ] A changed rule appears in the plan against the destination node.
- [ ] A node that runs a different Nebula version appears in the plan.
- [ ] A certificate inside the renewal window appears in the plan.
- [ ] A mesh that fails validation reads no node and exits 1.
- [ ] `--json` prints a machine readable plan that a workflow can post.
