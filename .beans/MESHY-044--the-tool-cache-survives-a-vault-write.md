---
# MESHY-044
title: The tool cache survives a vault write
status: completed
type: task
priority: high
tags:
    - vault
    - defect
created_at: 2026-11-18T01:40:00Z
updated_at: 2026-11-18T01:55:00Z
parent: MESHY-004
---

## Overview

A write to the vault rewrites the files of its record and leaves everything else in `.meshy` alone. The pinned tool cache lives in `.meshy/bin`, so it survives an `unlock` and an `issue`.

## Context / Background

`writeRecord` clears the whole directory before it writes the record. That was harmless while the directory held nothing but record files.

[ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) puts the pinned `nebula-cert` in `.meshy/bin/<version>/<os>-<arch>/`, and `ca.Find` reads it there. So every `unlock` and every `issue` deletes the tool that the next step needs. A CI run that installs the tool and then unlocks loses it before it can sign.

`lock` still removes the directory, because a person asked for the plaintext to go. That is the one command that should empty `.meshy`.

## Specification

| Entry in `.meshy` | On a vault write |
|---|---|
| a file the record names | written with mode 0600 |
| a file the record no longer names | removed, so a rotated key does not linger |
| `age.identity` | kept, because the write needs it |
| a directory, such as `bin/` | kept, with its contents |

## Technical Notes

- The record is the source of truth for files. A directory is not a record entry, so a write does not own it.
- The write still happens before the cleanup, so a failure leaves extra files rather than a missing one.
- A test places a file under `.meshy/bin/`, writes the vault, and asserts the file is still there.

## Reference Material

- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md): the cache path.
- [MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md): the command that introduced the write.
- [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md): the command that fills the cache.

## Acceptance Criteria

- [x] A file under `.meshy/bin/` survives a vault write.
- [x] A record file that the new record drops is removed.
- [x] `age.identity` survives a vault write.
- [x] Record files are written with mode 0600.
- [x] `lock` still removes the whole directory.
- [x] `go build ./...`, `go vet ./...`, and `go test ./...` pass.

## Summary of Changes

`writeRecord` no longer clears `.meshy`. It writes the record and then removes the files the record does not name.

### Files

- `internal/vault/vault.go`: the clear is gone, and `dropStaleFiles` removes a file that the record dropped. A directory and `age.identity` are left alone.
- `internal/vault/write_test.go`: four tests, one per rule in the Specification.

### Decisions

- The cleanup runs after the write, so a failure leaves an extra file rather than a missing one.
- `lock` still removes the whole directory. It is the command a person runs to get rid of the plaintext, and a cache is not a reason to keep it.

### Deviations

None. The defect came from [MESHY-005](MESHY-005--unseal-the-vault-for-a-run.md) and was harmless until [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md) put the tool cache in `.meshy/bin`.
