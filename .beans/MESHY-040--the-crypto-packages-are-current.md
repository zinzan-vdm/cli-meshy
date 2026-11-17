---
# MESHY-040
title: The crypto packages are current
status: completed
type: task
priority: high
tags:
    - toolchain
    - dependencies
    - security
created_at: 2026-11-18T00:12:00Z
updated_at: 2026-11-18T00:25:00Z
---

## Overview

`golang.org/x/crypto` and `golang.org/x/term` move to their current releases. The pins that held them back existed only because the build was on Go 1.22.

## Context / Background

Both modules were pinned to the last release that built on Go 1.22: `x/crypto` at v0.31.0 and `x/term` at v0.27.0. That floor is gone. The build is on 1.27.4.

The reason to move is the same as the toolchain, and the stakes are higher. `x/crypto` supplies `argon2` and `blake2b`, which are the vault's key derivation. A pin that is a year of releases behind cannot take a fix to those primitives without a deliberate move.

Neither module is a direct dependency in the sense of being chosen. `age` needs `x/crypto` for its primitives, and the SSH reader needs it for key parsing. The versions follow from what those callers can use.

## Specification

- `golang.org/x/crypto` is at v0.59.0, the current release.
- `golang.org/x/term` is at v0.48.0, the current release.
- `filippo.io/age` keeps the release it is on. `gopkg.in/yaml.v3` keeps its release. Neither is in this ticket.
- `go.mod` names 1.27.4 before and after. A dependency that demands a higher floor is a finding to report, not a change to make here.
- The `test` target, `go vet`, and `gofmt` stay clean.

## Technical Notes

- `x/term` reads the vault passphrase from a terminal. Its behaviour on a pipe and on a closed stdin is the part that matters, and the vault tests cover both.
- `argon2` and `blake2b` have no versioned API. A version move cannot change their interface, so the vault tests are a true check.
- The SSH reader parses keys through `x/crypto/ssh`. That package does change between releases. A compile failure there is the signal, and it is not a reason to stay pinned.
- The binary size in [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) is quoted from the current build. If the move changes it enough to matter, the table follows.

## Reference Material

- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md): the module count and size comparison this ticket can move.
- Go vulnerability database: <https://pkg.go.dev/vuln/>

## Acceptance Criteria

- [x] `go.mod` requires `golang.org/x/crypto` v0.59.0.
- [x] `go.mod` requires `golang.org/x/term` v0.48.0.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass.
- [x] The vault unlocks with three shares and refuses two, on the new primitives.
- [x] The `go` directive still names 1.27.4.
- [x] `beans check` passes and `write-tism` reports no finding.

## Summary of Changes

`golang.org/x/crypto` moved from v0.31.0 to v0.59.0, and `golang.org/x/term` from v0.27.0 to v0.48.0. `golang.org/x/sys`, which both depend on, moved from v0.28.0 to v0.50.0 as an indirect requirement.

### Files

- `go.mod`, `go.sum`: the three version bumps. The `go` directive still names 1.27.4, and the graph is still 13 modules.
- `docs/adr/0008-the-nebula-tools-do-the-signing.md`: the size row says 9.8 MB, and the text separates what the toolchain cost from what the dependency cost.
- `.beans/MESHY-039--the-build-uses-a-supported-go-release.md`: its Summary records the binary growth that the toolchain caused.

### Decisions

- Both modules move together. The same floor pinned `x/term`, and reading a passphrase is on the vault path. Leaving it behind would keep the old floor alive for no gain.
- The move stops at these three modules. `age` and `yaml.v3` are not in this ticket.

### Deviations

`golang.org/x/sys` moved without being named in the Specification. `x/crypto` and `x/term` both require the newer release, so the indirect requirement follows them, and `go mod tidy` set it.

### Measurements

- `gofmt -l .` is silent, `go build ./...` and `go vet ./...` pass, `go test ./...` passes.
- 99 test results, the same as before the move.
- The vault opens with three shares and refuses two, on `argon2` and `blake2b` from the new release.
- The binary grows 132 KB, from 9,631,520 to 9,764,010 bytes. The toolchain move accounted for the larger part, and ADR 0008 holds both numbers.
