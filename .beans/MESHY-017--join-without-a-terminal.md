---
# MESHY-017
title: A person joins without a terminal
status: scrapped
type: epic
priority: normal
tags:
    - bundle
    - user-nodes
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

`meshy bundle <node>` produces one archive that a person can install on their own machine. The archive holds the certificate, the key, the config, the Nebula binary, and an installer for their platform. This epic delivers the archive and the installers.

## Context / Background

A user node cannot be reached over SSH, so the control plane cannot configure it. That makes the archive the entire interface between the mesh and the person, and it must work without a shell, on macOS, Linux, and Windows.

## Specification

```
meshy bundle <node> [--no-binary] [--mesh <dir>]
```

Contents of `bundles/<mesh>/<node>.tar.gz`:

| File | Purpose |
|---|---|
| `config.yml` | the rendered config for the node |
| `ca.crt`, `<node>.crt`, `<node>.key` | identity and trust |
| `nebula` or `nebula.exe` | the pinned binary, unless `--no-binary` |
| `install.sh`, `install.command`, `install.ps1` | one per platform |
| `README.txt` | five lines: unzip, run the installer, connect |

The archive is mode 0600 and the command prints its path. Delivery is out of band.

## Technical Notes

- The binary comes from the cache that `meshy install` fills.
- The platform comes from the node's `platform` field, so one archive serves one machine.
- The installer clears the macOS quarantine attribute on the binary.

## Reference Material

- [MESHY-019](MESHY-018--a-person-joins-with-one-archive.md): the installers.
- [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md): the binary cache.
- [docs/questions/0001](../docs/questions/0001-devices-without-nebula.md): phones, which are out of scope.

## Acceptance Criteria

- [ ] The archive holds the six items in the table.
- [ ] The config inside the archive is the one the node needs, byte for byte.
- [ ] The archive has mode 0600.
- [ ] `--no-binary` leaves the binary out.
- [ ] A user node on each supported platform gets the matching binary.
- [ ] A node that is not `kind: user` is refused with a message that says why.

## Reasons for Scrapping

The epic held one deliverable. An archive nobody can install does not join a mesh, so the bundle and the installer are one task. The work is in [MESHY-018](MESHY-018--a-person-joins-with-one-archive.md).
