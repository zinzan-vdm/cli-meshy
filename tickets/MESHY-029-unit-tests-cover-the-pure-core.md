---
id: MESHY-029
title: Unit tests cover the pure core
type: task
status: completed
priority: high
parent: MESHY-028
tags: [tests, rendering, rules]
---

## Overview

Golden file tests for the Nebula config and table tests for the rule expansion. Both are pure functions, so both are proved without a node, a network, or a certificate.

## Context / Background

A wrong config is the most expensive failure in the project. It is also the easiest to test, because the renderer takes values and returns bytes. The same holds for the rule expansion, which turns a flat list into one firewall per node.

## Specification

Renderer tests:

| Case | Assertion |
|---|---|
| a lighthouse node | `am_lighthouse: true`, `listen.port` equals the lighthouse port |
| a server node | `am_lighthouse: false`, `static_host_map` holds every lighthouse |
| two trusted authorities | `pki.ca` holds both, in `ca.trust` order |
| a user node | the same config shape as a server node, no SSH field anywhere |
| a lighthouse with no `public_addr` | an error that names the node |

Golden files live in `internal/nebula/testdata/<case>.yml`. A failing test prints the diff.

Rule expansion tests hold one row per case in MESHY-010. The rows are a group rule, a host rule, `any`, a port range, `any/icmp`, duplicate collapse, an empty result, and an unknown group.

Planner tests: a mesh with no difference, a changed rule, a stale certificate, and a binary mismatch.

## Technical Notes

- Golden files are written with `-update` and committed, so a change to the output is a visible diff in review.
- The renderer takes a clock value, so a test fixes the date and the bytes stay stable.
- The expansion test compares structs, not text, so it fails on the meaning and not on the order.

## Reference Material

- [MESHY-028](MESHY-028-tests-prove-the-claims.md): the parent epic.
- [MESHY-009](MESHY-009-render-the-nebula-config.md): the renderer.
- [MESHY-010](MESHY-010-expand-rules-into-a-firewall.md): the expansion.
- [docs/patterns/0002](../docs/patterns/0002-renderers-are-pure.md): renderers are pure.

## Acceptance Criteria

- [x] Four golden files cover the renderer cases in the table.
- [x] Two renders of one input are byte identical.
- [x] A change to a golden file is the only way to change the expected bytes.
- [x] Ten expansion cases pass, one per row in MESHY-010.
- [x] The whole package runs in under one second.
- [x] No test reads a certificate, a clock, or a network.
- [x] Three refusals are tested: no public address, no lighthouse, unknown group.

The planner cases move to MESHY-012, because the planner does not exist yet.

## Summary of Changes

- `internal/nebula/render_test.go`: golden comparison with a `-update` flag, determinism, and refusals.
- `internal/nebula/firewall_test.go`: ten expansion cases, three refusals, and stability.
- `internal/nebula/testdata/`: four golden files.
