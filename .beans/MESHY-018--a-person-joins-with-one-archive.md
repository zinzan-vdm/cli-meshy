---
# MESHY-018
title: A person joins the mesh with one archive
status: todo
type: task
priority: high
tags:
    - bundle
    - installer
    - platforms
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-17T20:54:22Z
blocked_by:
    - MESHY-005
    - MESHY-009
    - MESHY-021
---
## Overview

`meshy bundle <node>` writes one archive that holds the config, the identity, the pinned binary, and a small installer for the node's platform. A person unpacks it, runs one file, and the machine joins the mesh.

## Context / Background

A user node cannot be reached over SSH, so the control plane cannot configure it. The archive is the entire interface between the mesh and the person.

The person who receives it may not read a terminal. Everything the machine needs must be inside the archive, in the order the installer expects, with a name that says what it is. That includes the privileged step: a launchd daemon on macOS, a service on Windows, a systemd unit on Linux.

## Specification

```
meshy bundle <node> [--no-binary] [--mesh <dir>]
```

Contents of `bundles/<mesh>/<node>.tar.gz`, flat at the top level:

| File | Purpose |
|---|---|
| `config.yml` | the rendered config for the node |
| `ca.crt`, `<node>.crt`, `<node>.key` | identity and trust |
| `nebula` or `nebula.exe` | the pinned binary, unless `--no-binary` |
| `install.sh`, `install.command`, `install.ps1` | one per platform |
| `README.txt` | five lines: unzip, run the installer, connect |

| Platform | Installer | What it does |
|---|---|---|
| Linux | `install.sh` | copy to `/etc/nebula/meshy-<mesh>/`, install the unit, enable and start |
| macOS | `install.command` | copy to `/etc/nebula/meshy-<mesh>/`, install a launchd plist, load it |
| Windows | `install.ps1` | copy to `C:\ProgramData\nebula\meshy-<mesh>\`, run `nebula -service install`, start |

Every installer checks the rights it needs, copies the files, sets the modes, installs the service as `meshy-<mesh>`, and prints one line.

| Condition | Behaviour |
|---|---|
| node is `kind: server` | exit 1, `web-1 is a server node, apply configures it` |
| vault is locked | exit 1, `the vault is locked, run meshy unlock` |
| certificate inside the renewal window | renew first, then build |
| binary not in the cache | fetch it through the install path, then build |

The archive is mode 0600 and the command prints its path. Delivery is out of band.

## Technical Notes

- Built in memory and written once, so a partial archive never reaches disk.
- The tar header sets mode 0600 on the key and 0755 on the binary.
- The installer templates live in `internal/scaffold/installers/` and are embedded with `go:embed`.
- The mesh name and the interface are substituted at bundle time.
- The Linux unit is `nebula@meshy-<mesh>.service`, which matches the name `apply` reloads.
- The binary comes from the cache that `meshy install` fills, so a user node and a server node get identical bytes.
- The macOS installer clears the quarantine attribute before the first run.
- The Windows script is signed only when the project has a certificate. Until then the README says what to expect.
- A test unpacks the archive and compares every file with its source.

## Reference Material

- [MESHY-009](MESHY-009--a-node-gets-the-config-it-must-run.md): the config inside the archive.
- [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md): the binary cache.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the unit name that both sides share.
- [docs/questions/0001](../docs/questions/0001-devices-without-nebula.md): phones, which are out of scope.

## Acceptance Criteria

- [ ] The archive holds the config, the trust bundle, the certificate, the key, the binary, and one installer.
- [ ] `config.yml` matches the rendered config for the node, byte for byte.
- [ ] The key has mode 0600 inside the archive, and the archive on disk has mode 0600.
- [ ] The installer puts the files in place, installs the service, and starts it.
- [ ] The Linux installer creates the unit that `apply` reloads.
- [ ] The macOS installer clears the quarantine attribute and loads a plist that starts at boot.
- [ ] The Windows installer registers and starts a service.
- [ ] An installer without the rights it needs exits with a message that names them.
- [ ] A server node, or a locked vault, is refused with a message that says why.
- [ ] `--no-binary` leaves the binary out, and the installer then fetches the pinned release and checks its checksum.
