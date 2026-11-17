---
id: MESHY-018
title: Build a bundle for a user node
type: task
status: todo
priority: normal
parent: MESHY-017
tags: [bundle]
---

## Overview

`meshy bundle <node>` assembles the archive a user node needs and writes it to `bundles/<mesh>/<node>.tar.gz`. It is the only artefact a person outside the mesh ever handles.

## Context / Background

The person who receives the archive may not read a terminal. Everything the machine needs must be inside the archive, in the order the installer expects, with a name that says what it is.

## Specification

```
meshy bundle <node> [--no-binary] [--mesh <dir>]
```

| Condition | Behaviour |
|---|---|
| node is `kind: server` | exit 1, `web-1 is a server node, apply configures it` |
| vault is locked | exit 1, `the vault is locked, run meshy unlock` |
| certificate inside the renewal window | renew first, then build |
| binary not in the cache | fetch it through the install path, then build |

The archive layout is flat: every file sits at the top level, so a person who unpacks it sees six files and no directories.

## Technical Notes

- Built in memory and written once, so a partial archive never reaches disk.
- The tar header sets mode 0600 on the key and 0755 on the binary.
- The `README.txt` names the installer for each platform.
- A test unpacks the archive and compares every file with its source.

## Reference Material

- [MESHY-017](MESHY-017-join-without-a-terminal.md): the parent epic.
- [MESHY-019](MESHY-019-ship-a-platform-installer.md): the installer inside the archive.

## Acceptance Criteria

- [ ] The archive holds the six files at the top level.
- [ ] `config.yml` matches the rendered config for the node.
- [ ] The key has mode 0600 inside the archive.
- [ ] A server node is refused with a clear message.
- [ ] A locked vault is refused with a clear message.
- [ ] An expiring certificate is renewed before the archive is built.
- [ ] The archive file on disk has mode 0600.
