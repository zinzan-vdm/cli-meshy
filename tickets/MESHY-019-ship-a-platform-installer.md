---
id: MESHY-019
title: Ship a platform installer
type: task
status: scrapped
priority: normal
parent: MESHY-017
tags: [bundle, installers]
---

## Overview

Each bundle carries a small installer for its platform. The installer puts the files in place, installs the service, and starts it. A person runs one file and the machine joins the mesh.

## Context / Background

Nebula needs a privileged step on every platform: a launchd daemon on macOS, a service on Windows, and a systemd unit on Linux. A person who is not a system administrator will not do that by hand.

## Specification

| Platform | File | What it does |
|---|---|---|
| Linux | `install.sh` | copy to `/etc/nebula/meshy-<mesh>/`, install the unit, enable and start |
| macOS | `install.command` | copy to `/etc/nebula/meshy-<mesh>/`, install a launchd plist, load it |
| Windows | `install.ps1` | copy to `C:\ProgramData\nebula\meshy-<mesh>\`, run `nebula -service install`, start |

Every installer:

1. checks that it runs with the rights it needs, and says so if not
2. copies the files and sets the modes
3. installs the service under the name `meshy-<mesh>`
4. starts the service and prints one line: `meshy-<mesh> is running`

The installer prefers the binary beside it and downloads the pinned release only when the binary is absent. On macOS it clears the quarantine attribute before the first run.

## Technical Notes

- The templates live in `internal/scaffold/installers/` and are embedded with `go:embed`.
- The mesh name and the interface are substituted at bundle time.
- The Linux unit is `nebula@meshy-<mesh>.service`, which matches the name `apply` reloads.
- The Windows script is signed only when the project has a certificate. Until then the README says what to expect.

## Reference Material

- [MESHY-017](MESHY-017-join-without-a-terminal.md): the parent epic.
- [MESHY-014](MESHY-014-apply-the-config-to-a-node.md): the unit name that both sides share.

## Acceptance Criteria

- [ ] Each platform has an installer in the archive.
- [ ] The Linux installer creates the unit that `apply` reloads.
- [ ] The macOS installer loads a launchd plist that starts at boot.
- [ ] The Windows installer registers and starts a service.
- [ ] Every installer prints one line on success and a reason on failure.
- [ ] An installer with no rights exits with a message that names the rights.
- [ ] The macOS installer clears the quarantine attribute.

## Reasons for Scrapping

The installer is part of the archive a person receives, so it was one deliverable with the bundle. The work is in [MESHY-018](MESHY-018-a-person-joins-with-one-archive.md).
