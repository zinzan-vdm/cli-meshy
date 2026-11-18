---
tags:
  - bundle
  - install
  - nebula
---

# One archive carries every installer and the binary

`meshy bundle` writes one archive for one node. The archive holds all three installers and the pinned Nebula binary. It does not hold only the installer for the platform in the mesh file.

## Why all three installers

The person who receives the archive may forward it, or unpack it on another machine than the one the mesh file names. Three small scripts cost about ten kilobytes. One archive that works on any machine is a smaller support problem than a maintainer who must rebuild because the platform line was wrong.

The mesh file still names one platform per user node. That platform decides the binary in the archive and the archive name in the installer. A person on another platform runs the installer for that platform, and it downloads the release for itself.

## Why the binary is inside

A user node is a machine that the maintainer cannot reach. Every step that needs the network is a step that can fail in a place the maintainer cannot see. The binary comes from the cache that `meshy install` fills. A user node and a server node then run identical bytes, and a person needs no network to join.

`--no-binary` exists for a small archive and for a platform that the project publishes no release asset for. The installer then downloads the pinned release and checks it against the published digest. The checksum file and the base URL come from `internal/release`, so the installer cannot drift from `meshy install`.

## Consequences

The archive is about twenty megabytes with the binary, and it is written with mode 0600, because it holds a private key. Delivery is out of band and the command prints the path.

The Windows script is unsigned. The README tells the person to choose "Run anyway" until the project has a signing certificate. A signed script is a later ticket.

The Linux unit that the installer writes is `nebula@meshy-<mesh>.service`, which is the name `apply` reloads. The installer writes the unit itself, because `bootstrap` runs over SSH and a user node has no SSH.

## See also

- [0009](0009-the-bootstrap-script-owns-the-host.md): the unit and the sudoers rule for a server node.
- [../patterns/0017](../patterns/0017-a-bundle-is-built-in-memory-and-written-once.md): the one write.
