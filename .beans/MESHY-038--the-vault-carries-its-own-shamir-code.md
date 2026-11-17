---
# MESHY-038
title: The vault carries its own Shamir code
status: completed
type: task
priority: normal
tags:
    - vault
    - vendoring
    - licence
created_at: 2026-11-17T22:45:00Z
updated_at: 2026-11-17T23:05:00Z
parent: MESHY-004
---

## Overview

The vault splits the age identity into five shares and rebuilds it from three. That arithmetic now lives in the repository, under the licence of the code we take, so no module supplies it.

## Context / Background

[MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md) first used `github.com/hashicorp/vault/shamir`. That package is correct and maintained, but the module brings 514 modules into the build for one function.

The second attempt used `github.com/SSSaaS/sssa-golang`, which brings the graph to 14. That library has had no release since 2017.

The code we trust is one file of 6.5 KB. It is small enough to hold, and holding it removes the dependency question entirely. The licence is MPL-2.0, which a GPL-3.0 project may include.

## Specification

`internal/shamir` holds two functions with the upstream contract:

```go
func Split(secret []byte, parts, threshold int) ([][]byte, error)
func Combine(parts [][]byte) ([]byte, error)
```

- The file keeps the upstream copyright line and the `SPDX-License-Identifier` line, unchanged.
- A header above them names the upstream path, the module version, the licence, and the date the file was taken.
- The body of the file stays as upstream wrote it, so an update is a diff against a known version.
- `internal/vault/share.go` imports `internal/shamir`.
- No module supplies Shamir, so `go list -m all` reports the modules of the rest of the build and nothing else.
- `README.md` credits the upstream under Licence, with the path a reader needs to check the source.

## Technical Notes

- Upstream: `github.com/hashicorp/vault/shamir`, module version `v1.15.0`, file `shamir.go`, 6509 bytes.
- Licence: MPL-2.0. The file-level header governs this file. MPL-2.0 is compatible with GPL-3.0.
- The upstream file calls `mathrand.Seed(time.Now().UnixNano())`. Go deprecated that call in 1.20. It stays as upstream wrote it, because the x-coordinates it produces are public: each share carries its own coordinate. A change here would be a change to upstream code, and the pattern asks for a diff against upstream instead.
- Our tests go in `internal/shamir/shamir_test.go` and cover the behaviour the vault relies on. The upstream test file is not copied, because our tests must state what we need.
- The vault keeps its own share format, `meshy-<label>-<base64>`. This ticket changes the arithmetic behind it and nothing a person sees.

## Reference Material

- [MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md): the vault that uses this code.
- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault design.
- [0004](../docs/patterns/0004-vendored-code-keeps-its-upstream.md): the rule this ticket follows.
- Upstream source: <https://github.com/hashicorp/vault/blob/v1.15.0/shamir/shamir.go>

## Acceptance Criteria

- [x] `internal/shamir` holds `Split` and `Combine` with the upstream contract.
- [x] The file carries the upstream copyright line and SPDX line, unchanged.
- [x] A header names the upstream path, the version, the licence, and the date taken.
- [x] `go.mod` lists no Shamir module, and `go list -m all` reports 13 modules.
- [x] `internal/vault/share.go` imports `internal/shamir`.
- [x] A test proves that three of five shares rebuild the secret, and that two do not.
- [x] Tests prove the error cases: an empty secret, parts below the threshold, and parts of unequal length.
- [x] `README.md` credits the upstream under Licence.
- [x] `docs/patterns/0004` records the vendoring rule and the licence compatibility.
- [x] `gofmt`, `go build`, `go vet`, `go test`, `beans check`, and `write-tism` pass.

## Summary of Changes

The vault now splits and rebuilds the age identity with code in this repository. No module supplies Shamir, and the build drops from 14 modules to 13.

### Files

- `internal/shamir/shamir.go`: upstream `shamir.go` from `github.com/hashicorp/vault` at v1.15.0, with a header that names the path, the version, the licence, and the date taken. The body is byte for byte upstream, so an update is a diff.
- `internal/shamir/shamir_test.go`: five tests of the behaviour the vault needs.
- `internal/vault/share.go`: imports `internal/shamir` instead of `github.com/SSSaaS/sssa-golang`. The share format and the commands do not change.
- `go.mod`, `go.sum`: `github.com/SSSaaS/sssa-golang` is gone.
- `docs/patterns/0004-vendored-code-keeps-its-upstream.md`: the rule, the licence check, and why one file replaced two modules.
- `docs/patterns/TAGS`: `licence` and `vendoring`.
- `README.md`: the credit under Licence, with the upstream path and version.

### Decisions

- The upstream licence is MPL-2.0, not MIT. The file header says so, and MPL-2.0 is compatible with GPL-3.0, so the file stays.
- The body is copied unchanged, including `mathrand.Seed`, which Go deprecated in 1.20. The x-coordinates it produces are public, and a local edit would turn every future update into a merge.
- Our tests replace the upstream ones. They state the three-of-five threshold, the two-share failure, the randomness of each split, and the four error cases.

### Deviations

None. The 13 modules are the main module, the four direct dependencies, and the transitive modules that `age` and `yaml` bring.
