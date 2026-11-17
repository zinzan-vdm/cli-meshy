---
id: MESHY-020
title: Binaries on the nodes match the pin
type: epic
status: scrapped
priority: normal
tags: [install, binaries]
---

## Overview

`meshy install` fetches the Nebula release that `mesh.yml` pins, verifies its checksum, and puts it on the server nodes. `apply` never moves a binary, so the version on the mesh changes only when someone asks.

## Context / Background

A version bump is a change with a restart behind it. Mixing that into `apply` would make every rule change a potential upgrade, and every upgrade a potential outage in the middle of a config write. One command, one job.

## Specification

```
meshy install [--node <name>] [--source <dir>] [--mesh <dir>]
```

| Behaviour | Detail |
|---|---|
| fetch | the release for `nebula.version` from the Nebula releases, per platform |
| verify | the SHA256 from the release checksums, and fail on a mismatch |
| cache | `.meshy/bin/<version>/<os>-<arch>/` |
| push | only when the node runs a different version |
| report | one line per node, and the version that moved |

`--source` reads the archives from a local directory, so an air gapped install needs no network.

## Technical Notes

- The cache is the same one `bundle` reads, so a user node and a server node get identical bytes.
- A checksum mismatch removes the download and fails the run.
- The install restarts the unit, unlike `apply`, because the binary changed.

## Reference Material

- [MESHY-021](MESHY-021-the-nodes-run-the-pinned-release.md): the fetch and verify.
- [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): the apply that never installs.
- [Nebula releases](https://github.com/slackhq/nebula/releases): the source of the archives.

## Acceptance Criteria

- [ ] `meshy install` puts the pinned version on every server node.
- [ ] A node that already runs the pin is not touched.
- [ ] A checksum mismatch fails the run and leaves no file in the cache.
- [ ] `--source` installs from a local directory with no network.
- [ ] `apply` never changes a binary.
- [ ] The cache holds one directory per version and platform.

## Reasons for Scrapping

The epic held one deliverable, and one deliverable is a task. The work is in [MESHY-021](MESHY-021-the-nodes-run-the-pinned-release.md).
