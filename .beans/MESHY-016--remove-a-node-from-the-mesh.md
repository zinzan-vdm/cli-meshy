---
# MESHY-016
title: Remove a node from the mesh
status: completed
type: task
priority: normal
tags:
    - certificates
    - revocation
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-18T09:30:00Z
parent: MESHY-013
---
## Overview

When a node leaves `mesh.yml`, `apply` removes it from every config and pushes its fingerprint to every node's blocklist. The removed node stops working even though its certificate has not expired.

## Context / Background

Removal is the case that a certificate lifetime cannot cover. A certificate lasts a year, and a stolen laptop must stop working today. Nebula does not distribute blocklists through lighthouses, so the control plane must write the blocklist to every node.

Part of [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): a merge changes the mesh.

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

- [MESHY-013](MESHY-013--a-merge-changes-the-mesh.md): the parent epic.
- [MESHY-009](MESHY-009--a-node-gets-the-config-it-must-run.md): the config that carries the blocklist.
- [Nebula pki reference](https://nebula.defined.net/docs/config/pki/): the blocklist.

## Acceptance Criteria

- [x] A node removed from `mesh.yml` appears in the plan as a removal.
- [x] `apply` removes its fingerprint from every node's config and adds it to every blocklist.
- [ ] The removed node cannot handshake with any node afterwards. Deferred to [MESHY-031](MESHY-031--a-rule-that-is-written-is-enforced-on-a-live-mesh.md), which needs a live mesh. See Status.
- [x] Its certificate and key leave the vault.
- [x] A user node is revoked the same way, without a remote write.
- [x] Adding the node back issues a new key, and the old key stays blocked.

## Summary of Changes

| File | Change |
|---|---|
| `internal/revoke` | `Names` finds the certificates that the record holds and the file does not name. `List` reads their fingerprints with the pinned tool and sets an authority aside. `Text` writes the blocklist. |
| `internal/vault/node.go` | `Certificates`, `DeleteCertificate`, `SetRevoked`, and `Revoked`. The fingerprint stays in the record, so a block outlives the certificate. |
| `internal/nebula/render.go` | `pki.blocklist` names one fixed path, `BlocklistPath`. |
| `internal/apply/apply.go` | `Files.Blocklist`, and `blocklist.txt` is the first file that a run writes. |
| `internal/plan/plan.go` | `Input.Revoked`, and `KindRevoked`, so a removal is a row in the plan. |
| `docs/patterns/0014`, `0015` | A block outlives the certificate. The blocklist is a file that the config names. |
| `CONTEXT.md` | The terms Revocation and Blocklist. |

## Deviations

**The blocklist is a file, not a rendered list.** The ticket puts the fingerprints in `pki.blocklist` inside the config. That cannot work here. `plan` renders the config to compare hashes, and a plan run cannot open the vault, so it cannot know the fingerprints. A config that carries them would make the plan report a difference that it cannot explain, on every run. The config names a fixed path instead, and the run writes the file. [0015](../docs/patterns/0015-the-blocklist-is-a-file-the-config-names.md) records the rule.

**The fingerprint comes from the tool.** A Nebula fingerprint is not a digest of the file or of the certificate bytes. This was tested against v1.12.0: the SHA-256 of the DER body does not match the fingerprint that `nebula-cert print -json` reports. The run therefore calls the pinned tool, which it already needs for a renewal.

**The blocklist is durable.** The first cut built the blocklist from the certificates that the vault still held. The blocklist then shrank after the run deleted them, and a second run rewrote every node. The record now keeps `<name>.revoked`, which holds the fingerprint after the certificate leaves. [0014](../docs/patterns/0014-a-block-outlives-the-certificate.md) records the rule. The integration test caught this.

**The certificates leave the vault only after every node took the blocklist.** A run that failed somewhere keeps the record, and the next run retries. A vault that dropped a certificate before a node took the blocklist would leave that node trusting a peer that the mesh removed.

**The plan shows a removal only when the vault is open.** A pull request from a fork holds no shares. It cannot see the certificate that the mesh no longer names. The removal is still in the `mesh.yml` diff, and that is the file under review.

## Status

The third criterion is a claim about a live mesh: a node that is blocked cannot complete a handshake. This host has no `/dev/net/tun`, no container runtime, and 954 MB of memory, so the run that would prove it belongs to [MESHY-031](MESHY-031--a-rule-that-is-written-is-enforced-on-a-live-mesh.md). The mechanism is proved here. The fingerprint of the removed certificate reaches the blocklist of every server node, and the certificate and key leave the vault.

## Verification

`TestApplyRevokesANodeThatLeftTheMesh` asserts the blocklist on every node, the report line, and the vault. `TestApplyKeepsTheVaultWhenANodeFails` asserts the order. `TestApplyRenewsAndRevokesWithTheRealTool` revokes a real certificate that the tool signed and asserts the blocklist holds the fingerprint that the tool prints.
