---
# MESHY-021
title: The nodes run the pinned Nebula release
status: todo
type: task
priority: normal
tags:
    - install
    - binaries
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T20:54:22Z
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
| 2 | for each platform in the mesh, fetch `nebula-<os>-<arch>.tar.gz` |
| 3 | compare the SHA256 with the published checksums |
| 4 | unpack `nebula` into `.meshy/bin/<version>/<os>-<arch>/` |
| 5 | upload to each server node under `/usr/local/bin/nebula` |
| 6 | restart `nebula@meshy-<mesh>` on the nodes that changed |
| 7 | print the version that moved and the nodes that moved |

| Failure | Behaviour |
|---|---|
| version not found | exit 1, `no release v9.9.9` |
| checksum mismatch | exit 1, `checksum mismatch for nebula-linux-amd64.tar.gz` |
| node upload fails | report the node, continue, exit 1 |

## Technical Notes

- Downloads use the release URL from the Nebula project, with no API token.
- The unpack keeps only the binary. The archive holds `nebula` and `nebula-cert`, and `nebula-cert` is never needed on a node.
- The restart is `systemctl restart`, not reload, because the binary changed.

## Reference Material

- [MESHY-018](MESHY-018--a-person-joins-with-one-archive.md): the bundle that reads this cache.

## Acceptance Criteria

- [ ] The pinned version lands on every server node.
- [ ] A node on the pin is not restarted.
- [ ] A checksum mismatch fails and leaves nothing in the cache.
- [ ] `--source` reads local archives and makes no network call.
- [ ] `--node` limits the run to one node.
- [ ] The cache path is `.meshy/bin/<version>/<os>-<arch>/nebula`.
- [ ] Only the binary is unpacked, never `nebula-cert`.
