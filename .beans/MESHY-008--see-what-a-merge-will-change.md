---
# MESHY-008
title: A maintainer can see what a merge will change
status: completed
type: epic
priority: high
tags:
    - plan
    - cli
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-18T01:05:00Z
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

- [x] A mesh with no differences exits 0 and prints one line per node.
- [x] A changed rule appears in the plan against the destination node.
- [x] A node that runs a different Nebula version appears in the plan.
- [x] A certificate inside the renewal window appears in the plan.
- [x] A mesh that fails validation reads no node and exits 2.
- [x] `--json` prints a machine readable plan that a workflow can post.

## Summary of Changes

Closed after the fact. Every task in this epic is completed or scrapped, and each acceptance criterion is met by a merged change.

- [MESHY-009](MESHY-009--a-node-gets-the-config-it-must-run.md) renders the config for one node.
- [MESHY-011](MESHY-011--a-maintainer-sees-what-a-node-runs.md) reads a live node.
- [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md) reports the differences and prints `--json`.
- [MESHY-010](MESHY-010--expand-rules-into-a-per-node-firewall.md) was scrapped. The rendering task took its work.

The last criterion said a validation failure exits 1. It exits 2, because [MESHY-012](MESHY-012--a-maintainer-sees-the-plan.md) set the contract to 0 for a match, 1 for a difference, and 2 for a run that never started. The criterion is corrected here.
