---
id: MESHY-002
title: A maintainer starts a mesh repository
type: task
status: completed
priority: high
parent: MESHY-001
tags: [cli, scaffold]
---

## Overview

`meshy init` writes the four files that make a directory a mesh repository. The output is a valid mesh that a maintainer can edit and validate without reading the reference first.

## Context / Background

The repository is the mesh, so starting a mesh is starting a repository. The scaffold carries the defaults that the design settled. A 20 year authority, one year certificates, a 60 day renewal window, and a workflow that plans on a pull request and applies on merge.

Part of [MESHY-001](MESHY-001-describe-and-validate-a-mesh.md): a maintainer can describe and validate a mesh.

## Specification

```
meshy init [--name <name>] [--cidr <cidr>] [--lighthouse <node>] [--owner <handle>] [<dir>]
```

| Flag | Default |
|---|---|
| `--name` | the base name of the target directory |
| `--cidr` | `10.42.0.0/24` |
| `--lighthouse` | `core-1` |
| `--owner` | `owner` |

Files written, and nothing else:

| Path | Contents |
|---|---|
| `mesh.yml` | one lighthouse node, an empty rule list, the settled defaults |
| `.gitignore` | `.meshy/`, keys, certificates |
| `.github/CODEOWNERS` | the owner on `mesh.yml` and `secrets.age` |
| `.github/workflows/meshy.yml` | plan on a pull request, apply on merge, weekly schedule |

An existing file is left alone. A run that writes nothing exits 0 and says so.

## Technical Notes

- Templates are string constants in `internal/scaffold/scaffold.go`.
- `init` derives the interface name through `config.GenerateInterface`, so the scaffold and the validator agree.
- The workflow pins actions by commit SHA.

## Reference Material

- [MESHY-001](MESHY-001-describe-and-validate-a-mesh.md): the parent epic.
- [ADR 0004](../docs/adr/0004-the-pull-request-is-the-approval-gate.md): the pull request is the gate.
- [docs/mesh-yml.md](../docs/mesh-yml.md): the field reference.

## Acceptance Criteria

- [x] `meshy init` writes exactly four files.
- [x] The mesh file that `init` writes passes `meshy validate`.
- [x] A second run writes nothing and exits 0.
- [x] The interface name in the output matches `config.GenerateInterface`.
- [x] The test loads the generated file and asserts that validation returns no problems.

## Summary of Changes

- `internal/scaffold/scaffold.go`: the four templates and the write logic.
- `internal/scaffold/scaffold_test.go`: the generated mesh validates, and a second run writes nothing.
- `cmd/meshy/main.go`: the `init` command and its flags.
