---
tags:
  - nebula
  - keys
---

# The Nebula tools do the signing

`meshy` runs the pinned `nebula-cert` binary to create an authority, sign a certificate, and print one. It does not import `github.com/slackhq/nebula`.

## Why

The module raises the Go floor from 1.22 to 1.26, and it pulls gvisor, protobuf, and the Windows WireGuard driver into the build. `meshy` needs three subcommands of one tool, and the release tarball already carries that tool because `meshy install` pins it.

The print form is the read path as well. `nebula-cert print -json` gives the fingerprint, the issuer, the names, the networks, and the expiry, which is everything `plan` compares.

## Consequences

A signing step needs the binary on the machine that runs it. `meshy` looks in `.meshy/bin/<version>/<os>-<arch>/` first and then on `PATH`, so a mesh that installed its release never needs the tool on `PATH`.

The Go floor stays at 1.22, and the module graph stays at 13.

A flag rename reaches us as a failure with the tool's own message, not as a compile error. `nebula-cert sign` already renamed `-ip` to `-networks`, and v1.12.0 requires the new name.
