---
# MESHY-009
title: A node gets the config it must run
status: completed
type: task
priority: high
tags:
    - nebula
    - config
    - rendering
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T20:54:22Z
parent: MESHY-008
---
## Overview

A function turns one node, its certificate, and the mesh into the Nebula config that the node must run, firewall included. The output is deterministic, so two runs on the same input produce the same bytes and the plan can compare by hash.

## Context / Background

Every node runs one Nebula process with one config. The config is derived, never hand written, so the mesh file stays the only source of truth.

The firewall is part of that config. A rule list that never becomes per-node inbound rules leaves the node open or shut, so the expansion belongs to this deliverable.

Part of [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): a maintainer can see what a merge will change.

## Specification

Input: the mesh, one node, and the node's certificate record. Output: a Nebula config as YAML.

| Section | Source |
|---|---|
| `pki.ca` | every authority in `ca.trust`, concatenated |
| `pki.cert`, `pki.key` | paths under `/etc/nebula/<mesh>/` |
| `pki.disconnect_invalid` | always true |
| `static_host_map` | every lighthouse, as `overlay-ip: ["underlay:port"]` |
| `lighthouse.hosts` | every lighthouse overlay address |
| `lighthouse.am_lighthouse` | true on a lighthouse node only |
| `lighthouse.port` | the port from the lighthouse entry |
| `listen.port` | the lighthouse port on a lighthouse, otherwise the default |
| `punchy.punch`, `punchy.respond` | true, so NAT mappings stay open |
| `tun.dev` | the mesh `interface` |
| `tun.unsafe_routes` | none in v1 |
| `firewall.inbound` | from the rule expansion |
| `firewall.outbound` | allow all |
| `firewall.conntrack` | defaults |
| `logging.level` | `info` |
| `stats` | off in v1 |

### The firewall

For each node, collect every rule whose `to` matches that node:

| `to` value | Expands to |
|---|---|
| a node name | that node |
| a group name | every node in the group |
| `any` | every node in the mesh |

| Rule field | Nebula field |
|---|---|
| `from` as a group | `group: <name>` |
| `from` as a node name | `host: <name>` |
| `from: any` | `host: any` |
| `ports` entry `443/tcp` | `port: 443`, `proto: tcp` |
| `ports` entry `8000-8100/tcp` | `port: 8000-8100`, `proto: tcp` |
| `ports` entry `any/icmp` | `proto: icmp`, no port |

Identical entries collapse. The order is sorted by source, then protocol, then port.
An empty list denies all inbound, which is the default for a node with no rules.

The renderer returns an error when a lighthouse has no `public_addr`, because the config would be unusable.

## Technical Notes

- `internal/nebula` holds the renderer, one file per section.
- The output is `map[string]any` marshalled by `yaml.v3` with sorted keys, so the bytes are stable.
- The renderer never reads a file and never writes one. It takes values and returns bytes.
- `internal/nebula/firewall.go` holds the expansion, and `firewall_test.go` holds its cases.
- A rule that names a group stays one entry even when the group holds twenty nodes.
- A test compares two renders of the same input byte for byte.

## Reference Material

- [MESHY-008](MESHY-008--see-what-a-merge-will-change.md): the parent epic.
- [Nebula firewall reference](https://nebula.defined.net/docs/config/firewall/): the rule fields.
- [Nebula config reference](https://nebula.defined.net/docs/config/): every key above.

## Acceptance Criteria

- [x] Two renders of the same input are byte identical.
- [x] A lighthouse node gets `am_lighthouse: true` and every other node gets false.
- [x] `static_host_map` holds every lighthouse with its underlay address.
- [x] `pki.ca`, `pki.cert`, and `pki.key` point into the node config directory.
- [x] `tun.dev` equals the mesh `interface`.
- [x] A lighthouse without `public_addr` returns an error.
- [x] Port ranges and `any/icmp` render in the Nebula form, and identical entries collapse.
- [x] Every rendered config passes `nebula -test` with the real binary.
- [x] A rule to a group appears once on every node in the group, as a group rule.
- [x] A rule to `any` appears on every node, and a node with no rule has an empty list.

## Summary of Changes

- `internal/nebula/render.go`: the document types and `Render`.
- `internal/nebula/render_test.go`: four golden cases, determinism, and three refusals.
- `internal/nebula/testdata/`: `lighthouse.yml`, `server.yml`, `user.yml`, `two-lighthouses.yml`.
- `internal/nebula/firewall.go`: `Expand`, the `Port` type, and source resolution.
- `internal/nebula/firewall_test.go`: ten expansion cases, three refusals, and a stability check.

Two changes from the specification. The trust bundle is written to `ca.crt` and referenced by path, not inlined, so rotation rewrites one file. ICMP rules carry no `port` field, because Nebula warns and ignores it. The sort key starts with the source name, so a group and a host sort by the name a person wrote.
