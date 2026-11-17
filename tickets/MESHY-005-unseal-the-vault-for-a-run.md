---
id: MESHY-005
title: Unseal the vault for a run
type: task
status: todo
priority: high
parent: MESHY-004
tags: [vault, keys]
---

## Overview

`meshy unlock` opens `secrets.age` with shares from a prompt or from the environment, and `meshy lock` removes the plaintext. A workflow can unseal, apply, and wipe without a person at a keyboard.

## Context / Background

The repository holds the authority key and every node key, so the file must be encrypted at rest. The runner is ephemeral, which makes a short-lived plaintext directory acceptable, provided the tool always removes it.

Part of [MESHY-004](MESHY-004-the-vault-seals-and-unseals.md): the vault seals and unseals the mesh keys.

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

- [MESHY-004](MESHY-004-the-vault-seals-and-unseals.md): the parent epic.
- [ADR 0005](../docs/adr/0005-the-vault-is-sealed-with-age-and-shares.md): the vault design.

## Acceptance Criteria

- [ ] Three shares open the vault and two fail with exit 1.
- [ ] `--from-env` reads shares from the environment and needs no prompt.
- [ ] `lock` removes `.meshy/` and every file under it.
- [ ] A second `unlock` without `--force` refuses.
- [ ] The plaintext directory and its files have modes 0700 and 0600.
- [ ] A test seals a vault, opens it with three shares, and compares the payload.
