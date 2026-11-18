---
tags:
  - bundle
  - install
---

# A bundle is built in memory and written once

In `internal/bundle`, build the whole archive in memory and write it with one call. Never stream the members to the destination file.

## Rationale

An archive holds a private key. A partial archive at the final path is a file that a person can copy and a script can fail on. The build returns bytes, so a failure leaves no file at all. `Write` removes the path when the write itself fails.

The same rule keeps the mode honest. The archive is created with mode 0600, and the key member carries 0600 in its tar header. Neither the file nor its content is world readable.

## Enforcement

`TestBuildWritesEveryFileFlat` reads the archive back and asserts the header mode of the key. `TestWriteSetsTheModeAndMakesTheDirectory` asserts the mode of the file on disk. `TestBundleWritesOneArchiveWithEveryFile` asserts that the command leaves no temporary file.

## See also

- [0001](0001-tests-need-no-root-or-network.md): the tests unpack an archive with no node.
- [0012](0012-the-release-is-verified-before-a-node-sees-it.md): the binary that the archive carries.
