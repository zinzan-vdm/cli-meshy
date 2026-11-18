---
tags:
  - install
  - binaries
---

# The release is verified before a node sees it

In `internal/release`, compare the archive with the published digest before anything reaches the cache. Then compare the binary with the digest that the same file publishes for that member.

## Rationale

A node runs a binary as root. A download that is not checked is a way to run code from anyone who can answer for that URL.

The project publishes one file, `SHASUM256.txt`, that names the archive and each member inside it:

```
44bf6ae5...  nebula-linux-amd64.tar.gz
52639c70...  nebula-linux-amd64.tar.gz/nebula
1def0b8f...  nebula-linux-amd64.tar.gz/nebula-cert
```

The first line proves the archive. The second proves the one file that the cache keeps. A cache that already holds a binary can then be judged without a download, and an unpack that wrote the wrong bytes is caught.

Nothing is written before the archive matches. A mismatch leaves the cache as it was, so a later run starts from the published file and not from a partial one.

## Enforcement

`TestFetchRejectsAnArchiveThatDoesNotMatch` and `TestLocalRejectsAnArchiveThatDoesNotMatch` assert the error and assert that no binary reached the cache. `TestFetchKeepsABinaryThatAlreadyMatches` serves no archive and proves the cache is judged by the member digest. `TestLocalKeepsTheRealArchive` reads the published archive and runs the binary it finds.

## See also

- [0013](0013-the-node-places-the-binary-through-one-exact-rule.md): the command that places the binary.
