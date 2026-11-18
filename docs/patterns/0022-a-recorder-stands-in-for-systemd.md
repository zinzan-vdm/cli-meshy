---
tags:
  - ci
  - systemd
  - tests
---

# A recorder stands in for systemd

A container has no systemd. A scenario that needs the unit state puts a `systemctl` recorder first on `PATH`. The recorder writes every call that changes a unit to a log. It answers the calls that read a unit. The scenario asserts on the log and on the unit state.

## Why a recorder and not a lighter unit

`bootstrap` writes a unit and calls `systemctl`. `install` restarts the unit. `apply` reads the unit state, restarts or reloads it, and reads it again. A product that skips those calls in a container would no longer be the product. The recorder keeps every call and gives the answer that a host gives.

## What the recorder models

- It records `daemon-reload`, `enable`, `start`, `restart`, `reload`, and `stop`.
- It writes no line for `is-active`, `cat`, and `status`, because a read changes nothing.
- It reports `active` after a start or a restart, and `inactive` before one. An idle unit then gets a restart and a running unit gets a reload.
- It exits 3 for an inactive unit, which is the status that `systemctl` uses.
- It answers `cat` from the unit file, so a host without the unit fails.

## Enforcement

`test/e2e/host/systemctl` is the recorder. The image puts it at `/usr/local/bin/systemctl`, which comes before every other directory on `PATH`. The runner asserts the exact lines in `/var/log/meshy-systemctl.log`. It asserts no new line for a second `bootstrap` and for an `install` that moves nothing.

## The ceiling

The recorder does not start Nebula. A container has no unit, so the scenario starts Nebula from a loop that waits for the files that `apply` writes. That loop is not the unit. A real virtual machine adds one assertion that the recorder cannot: the unit reports `active` and its process holds the tunnel.

## See also

- [0001](0001-tests-need-no-root-or-network.md): the tests that need no root.
- [ADR 0013](../adr/0013-the-empty-host-run-is-its-own-scenario.md): the scenario that uses this recorder.
