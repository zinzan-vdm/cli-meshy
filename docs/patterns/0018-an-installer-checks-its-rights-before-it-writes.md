---
tags:
  - install
  - bundle
---

# An installer checks its rights before it writes

In `internal/scaffold/installers`, every installer checks for root or administrator rights as its first step. It exits with a message that names the right when the check fails.

## Rationale

An installer copies a private key into `/etc` and registers a service. A run without the right fails halfway. It wrote some files and did not start the service, which leaves a host in a state that nobody can see from the outside. The check is one line at the top, so the failure is one message before any write.

The message names the right and the way to get it. `install.command` adds `sudo` in front of itself. `install.ps1` names PowerShell as administrator. The person who received the archive can act on the message without reading the script.

## Enforcement

`TestLinuxInstallerRefusesWithoutRoot` runs the Linux installer with a user id that is not root and asserts that nothing reached the file system. `TestMacInstallerRefusesWithoutRoot` runs the guard of the macOS installer. `TestWindowsInstallerRegistersTheService` asserts the administrator check in the Windows script, because this host cannot run PowerShell.

## See also

- [0001](0001-tests-need-no-root-or-network.md): the tests fake `id` instead of asking for root.
- [0017](0017-a-bundle-is-built-in-memory-and-written-once.md): the archive that carries the installer.
