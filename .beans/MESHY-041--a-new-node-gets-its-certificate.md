---
# MESHY-041
title: A new node gets its certificate
status: todo
type: task
priority: high
tags:
    - certificates
    - signing
    - apply
created_at: 2026-11-18T00:40:00Z
updated_at: 2026-11-18T00:40:00Z
parent: MESHY-013
blocked_by:
    - MESHY-006
---

## Overview

A node that `mesh.yml` names, and that holds no certificate, receives one. The pinned `nebula-cert` signs it with the active authority, and the key reaches the node without passing through git.

## Context / Background

The epic describes a step that issues a certificate, and no task owns it. Renewal covers a certificate that exists. A node with none is the first thing a new mesh needs, so the vertical stops at that gap.

The authority is created by [MESHY-006](MESHY-006--the-mesh-has-a-certificate-authority.md), which leaves the key in the vault. Signing needs the vault open, so `apply` runs after `meshy unlock`.

A host key is not a secret the repository can recover. It is generated for the node that will use it, and it lives there. A node that loses it gets a new certificate for the same address, and no peer notices.

## Specification

Signing uses the pinned binary, found the same way [MESHY-006](MESHY-006--the-mesh-has-a-certificate-authority.md) finds it:

```
nebula-cert sign -ca-key .meshy/ca-1.key -ca-crt .meshy/ca-1.crt \
  -name <node> -networks <ip>/<prefix> -groups <a,b> -duration <certificates.duration>
```

| Input | Source |
|---|---|
| name | `nodes[].name` |
| networks | `nodes[].ip` with the mesh prefix |
| groups | `nodes[].groups` |
| duration | `certificates.duration` |

- The key and the certificate are made in a temporary directory, uploaded to the node, and removed from the control host.
- The vault must be open. A closed vault exits 2 and names `meshy unlock`.
- The signed certificate is read back with `nebula-cert print -json` before it is uploaded, so a bad sign fails on the control host.

## Technical Notes

- `nebula-cert sign` writes `<name>.key` and `<name>.crt` unless `-out-key` and `-out-crt` name other paths. Both flags are set to the temporary directory.
- The issuer field of the printed certificate must name the active authority. That is the check that the right key signed it.
- The name must match the node name exactly. Nebula carries the name, and the config selects nodes by it.
- `-networks` replaced the deprecated `-ip` flag in v1.12.0, so the pinned release requires the new name.

## Reference Material

- [MESHY-006](MESHY-006--the-mesh-has-a-certificate-authority.md): the authority and the binary lookup.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write path this feeds.
- [ADR 0008](../docs/adr/0008-the-nebula-tools-do-the-signing.md): why the binary signs.

## Acceptance Criteria

- [ ] A node with no certificate receives one, and its issuer is the active authority.
- [ ] The private key reaches the node with mode 0600 and is absent from the control host after the run.
- [ ] The name, the networks, the groups, and the duration come from `mesh.yml`.
- [ ] A duplicate address or an empty name is refused before signing.
- [ ] A closed vault exits 2 and names `meshy unlock`.
- [ ] A certificate that does not name the active authority stops the run.
