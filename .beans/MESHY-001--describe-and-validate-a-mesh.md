---
# MESHY-001
title: A maintainer can describe and validate a mesh
status: completed
type: epic
priority: high
tags:
    - cli
    - config
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

A maintainer writes one file that describes a mesh, and the tool tells them whether that file is correct. This epic delivers the file format, the defaults, the validator, and the scaffold that starts a new mesh repository.

## Context / Background

Every later command reads the same file. If the file can hold contradictions, those commands are meaningless. Two nodes on one address, a lighthouse that is not a node, and a certificate that outlives its authority are all possible. The format and the validator come first because they are the contract for all other work.

## Specification

Two commands:

```
meshy init [flags] <dir>
meshy validate --mesh <dir>
```

`init` writes `mesh.yml`, `.gitignore`, `.github/CODEOWNERS`, and `.github/workflows/meshy.yml`. It never overwrites a file that exists.

`validate` loads `mesh.yml`, applies the defaults, and reports every problem in one run. Exit code 0 means valid. Exit code 1 means problems. Exit code 2 means bad usage.

The field reference is `docs/conf/0001-mesh-yml.md`.

## Technical Notes

- `internal/config` holds the types, the defaults, the loader, and the validator.
- `internal/scaffold` holds the templates that `init` writes.
- The loader rejects unknown fields, so a misspelled key fails instead of being ignored.
- The validator returns a sorted list of strings. It does not stop at the first problem.

## Reference Material

- [CONTEXT.md](../CONTEXT.md): the terms of the project.
- [docs/conf/0001-mesh-yml.md](../docs/conf/0001-mesh-yml.md): the field reference.
- [ADR 0003](../docs/adr/0003-the-repository-is-the-mesh.md): the repository is the mesh.
- [ADR 0006](../docs/adr/0006-overlay-addresses-are-explicit.md): addresses are explicit.

## Acceptance Criteria

- [x] `meshy init` writes four files and exits 0.
- [x] A second `meshy init` in the same directory writes nothing.
- [x] `meshy validate` exits 0 on the mesh that `init` wrote.
- [x] `meshy validate` reports every problem in one run, not one at a time.
- [x] A misspelled key fails the load.
- [x] The tests cover the validator, the duration parser, and the scaffold.
- [x] CI builds, vets, tests, scaffolds a mesh, and validates it.

## Summary of Changes

- `cmd/meshy/main.go`: the command, `init`, `validate`, and `version`.
- `internal/config/`: types, defaults, loader, validator, duration parser, tests.
- `internal/scaffold/`: the four files that `init` writes, with tests.
- `docs/conf/0001-mesh-yml.md`: the field reference.
- `.github/workflows/ci.yml`: build, vet, test, and a scaffold check.
