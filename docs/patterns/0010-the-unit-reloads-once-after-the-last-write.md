---
tags:
  - apply
---

# The unit reloads once, after the last write

In `internal/apply`, reload the unit one time, after the last write of a run. A run that writes nothing reloads nothing.

## Rationale

Nebula reads its config on `SIGHUP` and keeps its tunnels, so a reload is cheap. A reload after each file would load a config that is still half written.

## Enforcement

`TestRunWritesNothingWhenTheNodeMatches` asserts no reload with no write. `TestRunWritesEveryFileThroughATemporaryName` asserts one reload, after the last rename.

## See also

- [0009](0009-a-write-to-a-node-replaces-the-file-by-rename.md): the atomic write.
