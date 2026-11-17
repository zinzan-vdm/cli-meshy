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
| linked code | 7.4 MB for the whole CLI | 6.6 MB for a program that only imports the package |

The package links `x/crypto` (curve25519, argon2, blake2b, cryptobyte), `filippo.io/bigmod`, and `protobuf`. The toolchain floor is not the reason to refuse it: this repository builds on 1.27.4, which is above the 1.27.3 the module asks for. The weight is the reason.

`meshy` needs three subcommands of one tool, and the release tarball already carries that tool because `meshy install` pins it. The print form is the read path as well: `nebula-cert print -json` gives the fingerprint, the issuer, the names, the networks, and the expiry, which is everything `plan` compares.

## Consequences

A signing step needs the binary on the machine that runs it. `meshy` looks in `.meshy/bin/<version>/<os>-<arch>/` first and then on `PATH`, so a mesh that installed its release never needs the tool on `PATH`.

The module graph stays at 13, and the CLI stays at 7.4 MB.

A flag rename reaches us as a failure with the tool's own message, not as a compile error. `nebula-cert sign` already renamed `-ip` to `-networks`, and v1.12.0 requires the new name.

## Revisit when

A step needs signing inside a loop, or a policy the tool cannot express. Then the import earns its weight, and the toolchain floor becomes a decision we make on purpose.
