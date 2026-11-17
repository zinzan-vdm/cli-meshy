---
id: MESHY-010
title: Expand rules into a per-node firewall
type: task
status: todo
priority: high
parent: MESHY-008
tags: [rules, firewall]
---

## Overview

The flat rule list becomes one inbound firewall for each node. The expansion resolves groups to nodes, fans `any` out to every node, and produces a stable order so the plan can compare by hash.

## Context / Background

Nebula enforces rules on the destination, and it can name a source by certificate group. That means most rules compile to a single group rule, and only rules that name one node compile to a host rule.

## Specification

For each destination node, collect every rule whose `to` matches that node:

| `to` value | Expands to |
|---|---|
| a node name | that node |
| a group name | every node in the group |
| `any` | every node in the mesh |

Each collected rule becomes one inbound entry:

| Rule field | Nebula field |
|---|---|
| `from` as a group | `group: <name>` |
| `from` as a node name | `host: <name>` |
| `from: any` | `host: any` |
| `ports` entry `443/tcp` | `port: 443`, `proto: tcp` |
| `ports` entry `8000-8100/tcp` | `port: 8000-8100`, `proto: tcp` |
| `ports` entry `any/icmp` | `proto: icmp`, no port |

Identical entries are collapsed. The output is sorted by source, then protocol, then port, so the same input always renders the same bytes.

An empty firewall means deny all inbound, which is the intended default for a node with no rules.

## Technical Notes

- `internal/nebula/firewall.go` holds the expansion.
- Group rules are preferred over host rules: a rule that names a group stays one entry even when the group holds twenty nodes.
- A rule whose `to` names a single node but whose `from` is a group stays a group rule on that node.
- The expansion returns an error when a rule names a group that no node carries. The validator already rejects that case, so this is a second guard.

## Reference Material

- [MESHY-008](MESHY-008-see-what-a-merge-will-change.md): the parent epic.
- [MESHY-009](MESHY-009-render-the-nebula-config.md): the renderer that consumes this output.
- [Nebula firewall reference](https://nebula.defined.net/docs/config/firewall/): the rule fields.

## Acceptance Criteria

- [ ] A rule from a group to one node appears once on that node as a group rule.
- [ ] A rule to `any` appears on every node.
- [ ] A node with no matching rule has an empty inbound list.
- [ ] Identical entries collapse to one.
- [ ] The order is stable across runs.
- [ ] Port ranges and `any/icmp` render in the Nebula form.
- [ ] A rule that names an unknown group returns an error.
