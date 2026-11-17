---
tags:
  - vault
  - keys
---

# A write to the vault seals it with the key that opened it

A command that writes to the open vault seals `secrets.age` again with the identity in `.meshy/age.identity`. It never calls `vault.Seal`, because that call makes a new key, and the five shares that a person already holds would stop working.

`vault.Write` writes the open vault first and replaces `secrets.age` last, through a temporary name. A failure leaves the file that a person already has in place.

## Rationale

The shares are printed once and never again. A new key on every write would lose the vault at the first write after `ca init`.
