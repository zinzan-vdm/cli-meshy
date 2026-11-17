---
id: MESHY-016
title: Remove a node from the mesh
type: task
status: todo
priority: normal
parent: MESHY-013
tags: [certificates, revocation]
---

## Overview

When a node leaves `mesh.yml`, `apply` removes it from every config and pushes its fingerprint to every node's blocklist. The removed node stops working even though its certificate has not expired.

## Context / Background

Removal is the case that a certificate lifetime cannot cover. A certificate lasts a year, and a stolen laptop must stop working today. Nebula does not distribute blocklists through lighthouses, so the control plane must write the blocklist to every node.

Part of [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): a merge changes the mesh.

## Specification

| Step | Behaviour |
|---|---|
| 1 | find the nodes in the vault that are absent from `mesh.yml` |
| 2 | remove their entries from every `static_host_map` and firewall |
| 3 | add their fingerprints to `pki.blocklist` on every node |
| 4 | write and reload the nodes that changed |
| 5 | print `revoked <node> (<fingerprint>)` |

The removed node's certificate and key are deleted from the vault on the same run. A node that returns later is issued a new key.

## Technical Notes

- The blocklist is a list of SHA256 certificate fingerprints, one per line under `pki.blocklist`.
- The blocklist is part of the rendered config, so the hash comparison already covers a change.
- Removal is one way: the plan reports it, and there is no `unrevoke`.

## Reference Material

- [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-009](MESHY-009-a-node-gets-the-config-it-must-run.md): the config that carries the blocklist.
- [Nebula pki reference](https://nebula.defined.net/docs/config/pki/): the blocklist.

## Acceptance Criteria

- [ ] A node removed from `mesh.yml` appears in the plan as a removal.
- [ ] `apply` removes its fingerprint from every node's config and adds it to every blocklist.
- [ ] The removed node cannot handshake with any node afterwards.
- [ ] Its certificate and key leave the vault.
- [ ] A user node is revoked the same way, without a remote write.
- [ ] Adding the node back issues a new key, and the old key stays blocked.
