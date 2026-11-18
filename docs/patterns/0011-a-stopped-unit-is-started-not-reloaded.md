---
tags:
  - apply
  - systemd
---

# A stopped unit is started, not reloaded

In `internal/apply`, read the unit state after the writes. Reload a unit that is active, and restart a unit that is not.

## Rationale

`systemctl reload` cannot start a unit. It fails with `is not active, cannot reload`, and a unit that holds no `ExecReload` fails even while it runs with `Job type reload is not applicable`.

A node meets both states in normal use. `meshy bootstrap` enables the unit and leaves it stopped, because its restart is guarded by `systemctl is-active`. `meshy install` then restarts the node that changed, and at that moment no `config.yml` exists, so Nebula exits and the unit stays down.

`restart` covers both states. It starts a stopped unit and it clears a failed one. The sudoers rule that `meshy bootstrap` writes already names `reload` and `restart`, so this rule needs no new privilege.

## Enforcement

`TestRunStartsAUnitThatIsNotRunning` asserts a restart and no reload, after the last write. `TestRunWritesEveryFileThroughATemporaryName` asserts a reload and no restart on a unit that runs. `TestRunReportsAUnitThatIsNotActive` keeps the read back honest: a unit that refuses to come up is still a failure.

## See also

- [0010](0010-the-unit-reloads-once-after-the-last-write.md): one refresh, after the last write.
- [0009](0009-a-write-to-a-node-replaces-the-file-by-rename.md): the atomic write.
