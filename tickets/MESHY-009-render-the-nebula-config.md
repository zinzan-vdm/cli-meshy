---
id: MESHY-009
title: Render the Nebula config for one node
type: task
status: completed
priority: high
parent: MESHY-008
tags: [nebula, config, rendering]
---

## Overview

A function turns one node, its certificate, and the mesh into the Nebula config that the node must run. The output is deterministic, so two runs on the same input produce the same bytes and the plan can compare by hash.

## Context / Background

Every node runs one Nebula process with one config. The config is derived, never hand written, so the mesh file stays the only source of truth.

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

The renderer returns an error when a lighthouse has no `public_addr`, because the config would be unusable.

## Technical Notes

- `internal/nebula` holds the renderer, one file per section.
- The output is `map[string]any` marshalled by `yaml.v3` with sorted keys, so the bytes are stable.
- The renderer never reads a file and never writes one. It takes values and returns bytes.
- A test compares two renders of the same input byte for byte.

## Reference Material

- [MESHY-008](MESHY-008-see-what-a-merge-will-change.md): the parent epic.
- [MESHY-010](MESHY-010-expand-rules-into-a-firewall.md): the firewall section.
- [Nebula config reference](https://nebula.defined.net/docs/config/): every key above.

## Acceptance Criteria

- [x] Two renders of the same input are byte identical.
- [x] A lighthouse node gets `am_lighthouse: true` and every other node gets false.
- [x] `static_host_map` holds every lighthouse with its underlay address.
- [x] `pki.ca`, `pki.cert`, and `pki.key` point into the node config directory.
- [x] `tun.dev` equals the mesh `interface`.
- [x] A lighthouse without `public_addr` returns an error.
- [x] The rendered config parses as YAML and holds the sections in the table.
- [x] Every rendered config passes `nebula -test` with the real binary.

## Summary of Changes

- `internal/nebula/render.go`: the document types and `Render`.
- `internal/nebula/render_test.go`: four golden cases, determinism, and three refusals.
- `internal/nebula/testdata/`: `lighthouse.yml`, `server.yml`, `user.yml`, `two-lighthouses.yml`.

Two changes from the specification. The trust bundle is written to `ca.crt` and referenced by path, not inlined, so rotation rewrites one file. ICMP rules carry no `port` field, because Nebula warns and ignores it.
