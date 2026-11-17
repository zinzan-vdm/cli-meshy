---
tags:
  - nebula
  - keys
---

# The Nebula tools do the signing

`meshy` runs the pinned `nebula-cert` binary to create an authority, sign a certificate, and print one. It does not import `github.com/slackhq/nebula`.

## Why

The cost of the import is weight, not the language version.

| | the binary (today) | importing `nebula/cert` |
|---|---|---|
| go directive | 1.27.4 | 1.27.3 |
| modules in the graph | 13 | 47 |
| linked code | 9.8 MB for the whole CLI | 6.6 MB for a program that only imports the package |

The package links `x/crypto` (curve25519, argon2, blake2b, cryptobyte), `filippo.io/bigmod`, and `protobuf`. The toolchain floor is not the reason to refuse it: this repository builds on 1.27.4, which is above the 1.27.3 the module asks for. The weight is the reason.

Both sizes are measured on the same toolchain, so the comparison holds. The toolchain alone grew the CLI: on go1.22.2 it built to 7.4 MB, and on go1.27.4 with the same dependencies it builds to 9.6 MB. `go tool nm` finds 900 `fips140` symbols on 1.27.4 where 1.22 had none, so the standard library carries a FIPS 140 module now. Moving to current `x/crypto` adds another 132 KB.

`meshy` needs three subcommands of one tool, and the release tarball already carries that tool because `meshy install` pins it. The print form is the read path as well: `nebula-cert print -json` gives the fingerprint, the issuer, the names, the networks, and the expiry, which is everything `plan` compares.

## Consequences

A signing step needs the binary on the machine that runs it. `meshy` looks in `.meshy/bin/<version>/<os>-<arch>/` first and then on `PATH`, so a mesh that installed its release never needs the tool on `PATH`.

The module graph stays at 13. The CLI pays for the pinned toolchain, not for a certificate library.

A flag rename reaches us as a failure with the tool's own message, not as a compile error. `nebula-cert sign` already renamed `-ip` to `-networks`, and v1.12.0 requires the new name.

## Revisit when

A step needs signing inside a loop, or a policy the tool cannot express. Then the import earns its weight, and the toolchain floor becomes a decision we make on purpose.
