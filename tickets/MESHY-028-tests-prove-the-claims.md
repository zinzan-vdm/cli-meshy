---
id: MESHY-028
title: Tests prove the claims
type: epic
status: todo
priority: high
tags: [tests, ci]
---

## Overview

Three tiers of test prove the claims the design rests on. The first two run anywhere and cost milliseconds. The third needs root, a TUN device, and three containers, so it runs in CI only.

## Context / Background

The renderer and the rule expansion are pure functions, and they are the parts most likely to be wrong. They need no privileges to test. A handshake and a live reload need a real mesh, which needs root and a TUN device. That machine is a CI runner, not a laptop.

A mesh of three nodes in this repository's development environment exhausted memory and stopped the agent. The split below follows from that, not from taste.

## Specification

| Tier | Scope | Needs | Runs |
|---|---|---|---|
| 1 | validator, renderer, rule expansion, planner, bundle | nothing | every pull request, and a laptop |
| 2 | read, write, reload, idempotency | an in-process SSH server | every pull request |
| 3 | handshake, allowed port, denied port, live reload | root, `/dev/net/tun`, docker | a CI job only |

Tier 3 is draft. Tiers 1 and 2 are the work.

## Technical Notes

- Tier 1 and tier 2 files end in `_test.go` and carry no build tag.
- Tier 3 files carry `//go:build e2e` and run in a separate CI job.
- No test downloads a release. Fixtures hold the bytes.
- No test reads the clock or the network. Time enters through a parameter.

## Reference Material

- [docs/patterns/0001](../docs/patterns/0001-tests-need-no-root-or-network.md): the rule for every test file.
- [docs/patterns/0002](../docs/patterns/0002-renderers-are-pure.md): the rule for the renderer.
- [MESHY-009](MESHY-009-render-the-nebula-config.md): the renderer under test.
- [MESHY-014](MESHY-014-apply-the-config-to-a-node.md): the write path under test.

## Acceptance Criteria

- [ ] `go test ./...` passes on a machine with no network, no root, and no docker.
- [ ] The whole suite finishes in under ten seconds.
- [ ] The renderer has a golden file test.
- [ ] The rule expansion has a table test for every case in MESHY-010.
- [ ] The apply path is tested against an in-process SSH server.
- [ ] Tier 3 runs in its own CI job and does not block a merge while it is draft.
