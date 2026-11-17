---
id: MESHY-003
title: A maintainer sees every problem in the mesh file
type: task
status: completed
priority: high
parent: MESHY-001
tags: [cli, config, validation]
---

## Overview

`meshy validate` reads a mesh file and reports every problem in one run. The same checks run before `plan` and `apply`, so a file that fails validation is never applied.

## Context / Background

A mesh file can hold contradictions that only appear at apply time. Two nodes on one address, a lighthouse that is a user node, and a renewal window longer than the certificate lifetime all break a node. The validator moves those failures to the pull request, where they are cheap.

## Specification

```
meshy validate --mesh <dir>
```

| Result | Exit code | Output |
|---|---|---|
| valid | 0 | `mesh "<name>" is valid: N nodes, M rules` |
| problems | 1 | one line per problem, sorted |
| bad usage | 2 | flag error |

Checks:

| Area | Rule |
|---|---|
| schema | must equal the version the binary understands |
| name | starts with a letter, `a-z0-9-`, 32 characters at most |
| interface | starts with a letter, `a-z0-9-`, 15 characters at most |
| nebula.version | the form `v1.12.0` |
| cidr | an IPv4 network |
| ca.duration | more than zero, and longer than every certificate lifetime |
| ca.rotate_at | the form `80%` |
| ca.trust, ca.active | at least one entry, no duplicate, active must be trusted |
| certificates | each more than zero, renew window shorter than the shortest lifetime |
| node name | unique, and the form of the mesh name |
| node ip | unique, inside the cidr, not the network or broadcast address |
| node kind | `server` or `user` |
| server node | needs `ssh.user` and `ssh.host`, must not set `platform` |
| user node | needs `platform`, must not set `ssh` |
| lighthouses | at least one, each an existing server node with `public_addr` and a port in range |
| rules | `from` and `to` name a node, a group, or `any`, and every port matches `port/proto` |

## Technical Notes

- `Mesh.Validate` returns `[]string` and never stops early.
- The loader sets `KnownFields(true)`, so an unknown key fails the load with the file position.
- Durations parse `s`, `m`, `h`, `d`, and `y`. One year is 365 days.
- The validator sorts its output, so two runs produce the same text.

## Reference Material

- [MESHY-001](MESHY-001-describe-and-validate-a-mesh.md): the parent epic.
- [docs/mesh-yml.md](../docs/mesh-yml.md): the field reference and the full check list.

## Acceptance Criteria

- [x] A valid mesh exits 0 and prints the node and rule count.
- [x] A broken mesh exits 1 and prints every problem.
- [x] Two nodes on one address is a problem.
- [x] An address outside the cidr is a problem.
- [x] The network and broadcast addresses are problems.
- [x] A lighthouse that is not a server node is a problem.
- [x] A rule from a group that no node carries is a problem.
- [x] A renewal window longer than the certificate lifetime is a problem.
- [x] A certificate authority that is shorter than a certificate lifetime is a problem.
- [x] The tests cover 18 invalid meshes and one valid mesh.

## Summary of Changes

- `internal/config/config.go`: types, defaults, loader, and the validator.
- `internal/config/helpers.go`: address parsing and the last address of a network.
- `internal/config/duration.go`: the duration type and its parser.
- `internal/config/config_test.go`: the valid mesh, 18 invalid meshes, the duration parser, and the interface generator.
- `cmd/meshy/main.go`: the `validate` command.
