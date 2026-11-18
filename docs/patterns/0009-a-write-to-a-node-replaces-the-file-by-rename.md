---
tags:
  - apply
  - ssh
---

# A write to a node replaces the file by rename

In `internal/apply`, write every file to `<name>.tmp` in the same directory, then rename it into place.

## Rationale

A half written config stops Nebula on the next restart. A rename is atomic, so a reader sees the old file or the new one. A failed rename leaves the old file untouched.

## Enforcement

`TestRunWritesEveryFileThroughATemporaryName` in `internal/apply` asserts the temporary name and the rename for every file. `TestRunLeavesTheOldConfigWhenTheRenameFails` asserts that the old bytes survive a failed rename.

## See also

- [0010](0010-the-unit-reloads-once-after-the-last-write.md): the unit reloads once, after the last write.
