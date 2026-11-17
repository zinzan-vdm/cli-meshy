---
# MESHY-039
title: The build uses a supported Go release
status: completed
type: task
priority: high
tags:
    - toolchain
    - build
created_at: 2026-11-17T23:35:00Z
updated_at: 2026-11-18T00:05:00Z
---

## Overview

`go.mod` names a Go release that still receives fixes. The build, the tests, and CI all run on it, and nothing in the code changes to get there.

## Context / Background

The repository has said `go 1.22` since the first commit. Go supports the two most recent releases, so 1.22 stopped receiving standard library security fixes once 1.24 shipped.

The version was held back for a bad reason. An imported package would have raised it, and holding the line looked like a way to avoid that. The dependency question is settled separately, in [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md). An unsupported toolchain is a worse cost than a version number.

`go1.27.4` is the current stable release.

## Specification

- `go.mod` names `go 1.27.4`.
- The four direct dependencies stay: `filippo.io/age`, `golang.org/x/crypto`, `golang.org/x/term`, `gopkg.in/yaml.v3`.
- `.github/workflows/ci.yml` needs no edit. It reads `go-version-file: go.mod`, so it installs what the module names.
- `go mod tidy` runs after the bump, and the graph does not grow.
- No source file changes for the version itself.

## Technical Notes

- The toolchain is cached on this host, so `GOTOOLCHAIN=auto` resolves it without a download.
- `gofmt` output can differ between releases. A formatting change that the new release requires is part of this ticket, and it is still no behaviour change.
- `go vet` gains checks between releases. A finding it raises now is a defect this ticket fixes.
- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) states the current floor in its comparison table. It follows the bump.

## Reference Material

- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md): why the signing tool is a binary, and what the import would cost.
- Go release policy: <https://go.dev/doc/devel/release#policy>

## Acceptance Criteria

- [x] `go.mod` names `go 1.27.4`.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass on that release.
- [x] CI installs 1.27.4 from `go.mod` and passes, with no workflow edit.
- [x] The four direct dependencies are unchanged.
- [x] ADR 0008 names the current floor.
- [x] `beans check` passes and `write-tism` reports no finding.

## Summary of Changes

`go.mod` names `go 1.27.4`, the current stable release, and the whole build runs on it.

### Files

- `go.mod`: `go 1.22` becomes `go 1.27.4`. `go mod tidy` changes nothing else.
- `README.md`: the build line says Go 1.27.4 or later.
- `docs/adr/0008-the-nebula-tools-do-the-signing.md`: the comparison table names the current floor, and the text records that the floor is no longer a reason to refuse the import.

### Decisions

- The version is the current stable release, not the lowest one that works. A supported toolchain is the point of the ticket, so the newest release is the right target.
- The four direct dependencies stay. `golang.org/x/crypto` at v0.31.0 was pinned for the old floor, and moving it is separate work with its own risk.

### Deviations

None. `gofmt`, `go vet`, and the 99 test results are the same on the new release, so no source file changed.
