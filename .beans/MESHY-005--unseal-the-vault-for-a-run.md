---
# MESHY-005
title: Unseal the vault for a run
status: completed
type: task
priority: high
tags:
    - vault
    - keys
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-17T22:04:42Z
parent: MESHY-004
---

## Overview

`meshy unlock` opens `secrets.age` with shares from a prompt or from the environment, and `meshy lock` removes the plaintext. A workflow can unseal, apply, and wipe without a person at a keyboard.

## Context / Background

The repository holds the authority key and every node key, so the file must be encrypted at rest. The runner is ephemeral, which makes a short-lived plaintext directory acceptable, provided the tool always removes it.

Part of [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the vault seals and unseals the mesh keys.

## Specification

```
meshy unlock [--from-env] [--mesh <dir>]
meshy lock [--mesh <dir>]
```

| Input | Behaviour |
|---|---|
| no flags | prompt for shares until the threshold is met |
| `--from-env` | read `MESHY_SHARE_1` to `MESHY_SHARE_5`, use the ones that are set |
| fewer than three shares | exit 1, `need 3 shares, got N` |
| wrong shares | exit 1, `the shares do not open this vault` |

`unlock` writes plaintext to `<mesh>/.meshy/` and prints that path. `lock` removes the directory and prints `locked`.

`unlock` refuses when the directory already holds plaintext and `--force` is absent.

## Technical Notes

- `filippo.io/age` with a scrypt or X25519 recipient. The file key is the Shamir secret.
- Share format: `meshy-1-<base64>`, so a share names its index and is recognisable in a password manager.
- `.meshy/` is in `.gitignore` and the scaffold writes that line.
- File modes: 0700 on the directory, 0600 on every file.

## Reference Material

- [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md): the parent epic.
- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault design.

## Acceptance Criteria

- [x] Three shares open the vault and two fail with exit 1.
- [x] `--from-env` reads shares from the environment and needs no prompt.
- [x] `lock` removes `.meshy/` and every file under it.
- [x] A second `unlock` without `--force` refuses.
- [x] The plaintext directory and its files have modes 0700 and 0600.
- [x] A test seals a vault, opens it with three shares, and compares the payload.

## Summary of Changes

`meshy unlock` opens `secrets.age` with three of five shares and writes the plaintext to `<mesh>/.meshy/`. `meshy lock` removes that directory. Shares come from a hidden prompt or from `MESHY_SHARE_1` to `MESHY_SHARE_5`.

### Files

- `internal/vault/vault.go`: `Seal`, `Open`, `Unlock`, `Lock`, and the vault constants.
- `internal/vault/share.go`: the Shamir split and combine, and the `meshy-<label>-<base64>` share format.
- `internal/vault/record.go`: `Record`, `Encode`, and `Decode`. One record entry becomes one file.
- `internal/vault/vault_test.go`: 14 tests of the vault.
- `cmd/meshy/vault.go`: `cmdUnlock`, `cmdLock`, and the two share sources.
- `cmd/meshy/vault_test.go`: 4 tests of the commands.
- `cmd/meshy/main.go`: the dispatch of `unlock` and `lock`, and the usage text.
- `go.mod`, `go.sum`: the dependencies below.

### Dependencies

| Module | Version | Why |
|---|---|---|
| `filippo.io/age` | v1.2.1 | the sealed file |
| `github.com/hashicorp/vault` | v1.15.0 | the `shamir` package, standard library only |
| `golang.org/x/term` | v0.27.0 | a hidden prompt, so a share does not echo |

The go directive stays at 1.22. `go mod tidy` also removed the `// indirect` mark from `golang.org/x/crypto`, because `internal/sshx` imports it.

### Decisions

- The Shamir secret is the age X25519 identity string. Three shares rebuild the identity, and the identity unwraps the file key of `secrets.age`.
- The share label is the position in the set of five. The Shamir x-coordinate stays inside the share. A person can move a share between password manager entries and it still works.
- Shares that are well formed but do not open the vault report `the shares do not open this vault`. A share that is not in the `meshy` format reports `share N: "..." is not a meshy share`.
- An empty `.meshy/` directory does not block `unlock`, because it holds no plaintext.

### Deviations

None from the specification.
