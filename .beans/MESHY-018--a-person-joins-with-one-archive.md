---
# MESHY-018
title: A person joins the mesh with one archive
status: completed
type: task
priority: high
tags:
    - bundle
    - installer
    - platforms
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T11:28:50Z
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
| `blocklist.txt` | the fingerprints that the node refuses, which the config names |
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

- [x] The archive holds the config, the trust bundle, the certificate, the key, the binary, and one installer.
- [x] `config.yml` matches the rendered config for the node, byte for byte.
- [x] The key has mode 0600 inside the archive, and the archive on disk has mode 0600.
- [x] The installer puts the files in place, installs the service, and starts it.
- [x] The Linux installer creates the unit that `apply` reloads.
- [ ] The macOS installer clears the quarantine attribute and loads a plist that starts at boot.
- [ ] The Windows installer registers and starts a service.
- [x] An installer without the rights it needs exits with a message that names them.
- [x] A server node, or a locked vault, is refused with a message that says why.
- [x] `--no-binary` leaves the binary out, and the installer then fetches the pinned release and checks its checksum.

## Deviations

- Two criteria stay unticked. This host runs Linux, so `install.command` and `install.ps1` never ran here.
- The macOS and Windows installers are proved by structure. A test asserts their text, the paths they write, the plist with `RunAtLoad`, the quarantine step, the administrator check, and the service commands.
- The guard of `install.command` does run, because it is a shell script and it stops before the first macOS command.
- The Linux installer runs in a temporary prefix, so the test needs no root and touches no system file. A test doubles `id`, `systemctl`, and `ip`.
- The shell fetch path in `install.sh` is asserted, not executed, because a test has no network. The Go fetch path that fills the cache runs against a test server.
- A Windows node needs `--no-binary`. The project publishes a zip for Windows. `release.Supported` names only the platforms with a tar.gz, so `bundle` refuses the binary for that node and names the flag.
- The Windows installer registers the service under the name that `nebula -service install` uses, which is `nebula`. Linux and macOS use `meshy-<mesh>`.
- The contents table gained `blocklist.txt`. `internal/nebula` renders a fixed blocklist path, so an archive without the file gives a node that cannot read its blocklist.

## Summary of Changes

| File | Change |
|---|---|
| `cmd/meshy/bundle.go` | new: the command, the archive inputs, and the installer values |
| `cmd/meshy/certificate.go` | new: the signer that `apply` and `bundle` share, moved out of `apply.go` |
| `cmd/meshy/apply.go` | the run holds the shared signer, with no change of behaviour |
| `cmd/meshy/main.go` | the dispatch and the usage name `bundle` |
| `internal/bundle/bundle.go` | new: the archive, the modes, the one write, and the README |
| `internal/scaffold/installers/installers.go` | new: render the three installers from the templates |
| `internal/scaffold/installers/install.sh.tmpl` | new: the Linux installer |
| `internal/scaffold/installers/install.command.tmpl` | new: the macOS installer |
| `internal/scaffold/installers/install.ps1.tmpl` | new: the Windows installer |
| `internal/release/release.go` | new: `Cached`, so `bundle` reads the cache that a fetch verified |
| `internal/scaffold/scaffold.go` | `.gitignore` keeps `bundles/` out of git |
| `cmd/meshy/bundle_test.go` | new: the archive contents, the modes, the refusals, the renewal, the fetch |
| `cmd/meshy/bundle_installer_test.go` | new: run `install.sh`, and the guards that refuse without root |
| `internal/bundle/bundle_test.go` | new: the build, the modes, the write, and the README |
| `internal/scaffold/installers/installers_test.go` | new: the structure of the three installers |
| `internal/scaffold/scaffold_test.go` | new: `.gitignore` holds `bundles/` |
| `README.md` | the status, the join section, and the command table |
| `CONTEXT.md` | the bundle entry, and the new installer term |
| `docs/adr/0010-one-archive-carries-every-installer.md` | new: why one archive holds all three |
| `docs/patterns/0017-a-bundle-is-built-in-memory-and-written-once.md` | new: the one write |
| `docs/patterns/0018-an-installer-checks-its-rights-before-it-writes.md` | new: the rights check |
| `docs/adr/TAGS`, `docs/patterns/TAGS` | the bundle tag |
