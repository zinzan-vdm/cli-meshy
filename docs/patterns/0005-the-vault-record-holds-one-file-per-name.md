---
tags:
  - vault
  - keys
---

# The vault record holds one file per name

In the vault, one record entry is one file under `.meshy`, and the entry name is the file name. An authority and a node both hold `<name>.crt` and `<name>.key`, so the vault never mixes two keys.

A record entry never holds a path. `meshy` refuses a name with a slash, a name that is `.` or `..`, and the name of the file that seals the vault.

## Rationale

One entry per file keeps `unlock` small. It writes the record to disk, and `lock` removes the directory. The certificate and the key sit next to each other under the name that a person knows.
