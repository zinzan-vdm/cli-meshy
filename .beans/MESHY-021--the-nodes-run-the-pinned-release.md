---
# MESHY-021
title: The nodes run the pinned Nebula release
status: completed
type: task
priority: normal
tags:
    - install
    - binaries
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T02:05:00Z
parent: MESHY-043
---
## Overview

`meshy install` resolves the pinned release, verifies it, caches it, and pushes it to the server nodes that run something else. The mesh file stays the only place a version is written.

## Context / Background

Every node must run the same build, or the mesh behaves in ways that are hard to explain. A pin in `mesh.yml` plus a verified download gives that, without a package manager on the node.

## Specification

```
meshy install [--node <name>] [--source <dir>] [--mesh <dir>]
```

| Step | Behaviour |
|---|---|
| 1 | read `nebula.version` from `mesh.yml` |
| 2 | read the platform of each target with `uname -s` and `uname -m` |
| 3 | fetch `nebula-<os>-<arch>.tar.gz` and `SHASUM256.txt` for each platform in the run |
| 4 | compare the SHA256 of the archive with the line that names it |
| 5 | unpack `nebula` into `.meshy/bin/<version>/<os>-<arch>/` |
| 6 | upload it to each server node under `/usr/local/bin/nebula` |
| 7 | restart `nebula@meshy-<mesh>` on the nodes that moved |
| 8 | print the version that moved and the nodes that moved |

| Failure | Behaviour |
|---|---|
| version not found | exit 1, `no release v9.9.9` |
| checksum mismatch | exit 1, `checksum mismatch for nebula-linux-amd64.tar.gz` |
| platform with no asset | exit 1, `node core-1 runs linux/riscv128, which has no release asset` |
| node upload fails | report the node, continue, exit 1 |

The exit codes follow the contract that `apply` already uses.

| Code | Meaning |
|---|---|
| `0` | every target is on the pin |
| `1` | the release cannot be used, or a node failed |
| `2` | the run never started |

## Technical Notes

- Downloads use the release URL from the Nebula project, with no API token. The checksums file is `SHASUM256.txt` beside the archive.
- The unpack keeps only the binary. The archive holds `nebula` and `nebula-cert`, and `nebula-cert` is never needed on a node.
- The restart is `systemctl restart`, not reload, because the binary changed.
- A node reports its release as `Version: 1.12.0` while the pin reads `v1.12.0`, so both are compared without the leading `v`.
- The node user cannot write `/usr/local/bin`, and its sudoers rule holds two verbs today. [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md) gains a third command that names the exact source, destination, owner and mode:

  ```
  <user> ALL=(root) NOPASSWD: <systemctl> reload <unit>.service, <systemctl> restart <unit>.service, <install> -o root -g root -m 0755 <dir>/nebula.incoming /usr/local/bin/nebula
  ```

  The binary travels to `<dir>/nebula.incoming` through the write path of [MESHY-014](MESHY-014--apply-the-config-to-a-node.md), and one command places it. There is no wildcard and no shell.
- `install` reads the binary and replaces the target, so a running Nebula does not hold the old file busy.
- The cache is inside `.meshy`, so `meshy lock` empties it and the next run fetches again. A locked vault is not an error for this command.

## Reference Material

- [MESHY-018](MESHY-018--a-person-joins-with-one-archive.md): the bundle that reads this cache.
- [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md): the script that grants the privilege.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write path this reuses.

## Acceptance Criteria

- [x] The pinned version lands on every server node.
- [x] A node on the pin is not restarted.
- [x] A checksum mismatch fails and leaves nothing in the cache.
- [x] `--source` reads local archives and makes no network call.
- [x] `--node` limits the run to one node.
- [x] The cache path is `.meshy/bin/<version>/<os>-<arch>/nebula`.
- [x] Only the binary is unpacked, never `nebula-cert`.
- [x] The node user places the binary through one rule that names the exact source, destination, owner and mode.
- [x] A node that runs another release is restarted once, after the binary is in place.
- [x] A failed node does not stop the run, and the run exits 1.

## Summary of Changes

`meshy install` resolves the pin, verifies it, keeps it, and puts it on the server nodes that run another release.

### Files

- `internal/release/release.go`: `Asset`, `Normalize`, `Supported`, `Checksums`, `Fetch`, `Local`, and the unpack that keeps one member. It reads the archive and the member digests from the one file the project publishes.
- `internal/install/install.go`: `PlatformOf` reads `uname -s` and `uname -m`. `Run` moves one node at a time, reports a failure, and continues.
- `cmd/meshy/install.go`: the flags, the target list, the cache in `.meshy/bin/`, and the report.
- `internal/bootstrap/bootstrap.go`: the tool check gains `install`, and the sudoers rule gains the one command that places the binary.
- Tests: 33 new results, including a run of the whole command from a source directory.
- `docs/patterns/0012` and `0013`.
- `docs/adr/0009` records the third command and the upgrade path.
- README holds the new section and the corrected bootstrap list.

### Decisions

- The node reports its platform, and the run trusts the answer. A pin that a node cannot run is refused with `node core-1 runs linux/riscv64, which has no release asset`, rather than a binary that fails to start.
- The archive is checked before the unpack, and the binary is checked again after it. The second check is what lets a cache be judged without a download, and it catches an unpack that wrote the wrong bytes.
- `restart` and not `reload`, because the binary changed. `apply` keeps the reload path, because only the config changed.
- The binary travels on stdin as the node user, and one sudo command places it. The rule names the source, the destination, the owner, and the mode, so it holds no wildcard and no shell.
- The cache stays inside `.meshy`, so `meshy lock` empties it. A locked vault is not an error for this command.

### Deviations

- The ticket said the archive is fetched for each platform in the mesh. A server node holds no `platform` field, so the run asks each node instead. The result is the same for the two platforms the release publishes for a server.
- `windows` and `386` are refused even though the release publishes them, because a node runs Linux and the two names a node answers with are `amd64` and `arm64`.

### Measurements

- `gofmt -l .` silent, `go build ./...` and `go vet ./...` pass.
- 223 test results across 15 packages, 33 more than before.
- The real `nebula-linux-amd64.tar.gz` was read through `release.Local`: the cache holds a 22,424,339 byte binary, and that binary reports `Version: 1.12.0`. The test is gated on `MESHY_RELEASE_DIR`, so CI downloads nothing.
- The generated sudoers rule was written to a file and `visudo -cf` reported `parsed OK`.
- `sh -n` passes on the whole bootstrap script.
