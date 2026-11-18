---
# MESHY-025
title: Every node trusts the next authority
status: completed
type: task
priority: high
tags:
    - certificates
    - rotation
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T14:43:25Z
parent: MESHY-024
blocked_by:
    - MESHY-006
---
## Overview

`meshy ca rotate --phase 1` creates the next authority, adds it to `ca.trust`, and applies the new trust bundle to every node. The mesh keeps running, and every node still holds a certificate from the old authority.

## Context / Background

The authority key is the root of trust, and a mistake here locks every node out. Phase 1 is the safe half of the rotation: it adds trust without changing any certificate.

Every later phase refuses to run until phase 1 is done, so a maintainer cannot start in the middle.

Part of [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the trust anchor rotates without downtime.

## Specification

```
meshy ca rotate --phase 1 [--mesh <dir>]
```

| Step | Work |
|---|---|
| 1 | create `ca-2`, sealed into the vault |
| 2 | add `ca-2` to `ca.trust` in `mesh.yml` |
| 3 | apply the new trust bundle to every node |
| 4 | read back the authority list from every node |

| Refusal | Message |
|---|---|
| `ca.trust` already holds two authorities | `ca.trust already holds two authorities` |
| the vault is locked | `the vault is locked, run meshy unlock` |

The command prints the phase that comes next.

## Technical Notes

- The command edits `mesh.yml`, so the rotation produces a diff a reviewer can read.
- Phase 1 issues no certificate, so a node that is not reached keeps working.
- The read back at the end is the proof that every node took the new bundle.
- The next authority is named `ca-<n+1>`, so a rotation reads the same every time.

## Reference Material

- [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the parent epic.
- [MESHY-034](MESHY-034--every-node-holds-a-new-certificate.md): phase 2.
- [MESHY-006](MESHY-006--the-mesh-has-a-certificate-authority.md): the authority creation that phase 1 reuses.

## Acceptance Criteria

- [x] Phase 1 creates an authority and adds it to `ca.trust`.
- [x] The new trust bundle reaches every node.
- [ ] The mesh keeps working with every node on the old certificate.
- [x] Phase 1 refuses to run when `ca.trust` already holds two authorities.
- [x] The command prints the phase that comes next.

## Summary of Changes

| File | Change |
|---|---|
| `cmd/meshy/rotate.go` | new: `ca rotate --phase 1`. It refuses a trust of two and a locked vault, creates `ca-<n+1>` with the pinned tool, seals it, edits `ca.trust`, applies the bundle to every server node, reads the authority list back, and prints the next phase. |
| `cmd/meshy/ca.go` | the `ca` dispatch names the `rotate` subcommand. |
| `cmd/meshy/main.go` | the usage lists `ca rotate` and drops it from Planned. |
| `cmd/meshy/apply.go` | the authority file of a node is the trust bundle, so a node accepts every authority that `ca.trust` names. |
| `internal/vault/trust.go` | new: `TrustBundle` joins the certificate of each trusted authority into one file. |
| `internal/config/trust.go` | new: `AddTrust` appends one authority to `ca.trust` and changes no other byte of `mesh.yml`. |
| `internal/ca/next.go` | new: `Next` returns the name after the highest authority. |
| `internal/node/state.go` | `ParseCerts` reads every document that the tool prints. `State.Authorities` holds the bundle. |
| `internal/node/read.go` | `ReadAuthorities` returns the authority list from one node. |
| `internal/plan/plan.go` | `authorityFacts` compares the authority set on the node with `ca.trust`, not one name. |
| `docs/adr/0011-a-trust-bundle-is-one-file-of-certificates.md` | new: the probe of Nebula v1.12.0 and the decision that follows. |
| `docs/patterns/0020-an-edit-to-mesh-yml-keeps-every-other-byte.md` | new: the rule for an edit to a reviewed file. |
| `docs/conf/0001-mesh-yml.md` | `ca.trust` and `ca.active` name what a rotation does. |
| `README.md` | the rotate section, the status line, and the command table. |
| `docs/adr/TAGS`, `docs/patterns/TAGS` | the new tags. |

## Verification

`TestCARotatePhase1CreatesTheNextAuthority` proves that `ca-2` is sealed, that `ca.trust` holds both names, that `active` stays `ca-1`, and that both nodes hold the bundle and reload once.

`TestCARotatePhase1IssuesNoCertificate` proves that the tool is asked for an authority and never signs, and that the node keeps its certificate.

`TestCARotatePhase1RefusesATrustOfTwo`, `TestCARotatePhase1SaysUnlockWhenTheVaultIsClosed`, `TestCARotatePhase1ContinuesAfterANodeFails`, and `TestCARotatePhase1ReportsANodeThatDidNotTakeTheBundle` cover the refusals and the read back.

`TestAddTrustChangesOneLineAndKeepsTheRest` and `TestAddTrustKeepsEveryCharacterThatItDidNotEdit` prove the edit of a file that carries comments and blank lines.

`TestComputeReportsAnAuthorityThatTheNodeDoesNotTrust` and `TestComputeReportsNoChangeWhenTheNodeTrustsEveryAuthority` prove the plan on a bundle of two.

The command was also run against a real mesh directory with the pinned `nebula-cert` v1.12.0. It created `ca-2`, kept the comment and the blank lines of `mesh.yml`, and printed the refusal on the second run.

## Deviations

- The criterion "The mesh keeps working with every node on the old certificate" needs a live mesh. This host has no Docker and no `/dev/net/tun`, so no tunnel came up here. MESHY-031 proves it on a live mesh. The trust bundle that makes it true is proved in [ADR 0011](../docs/adr/0011-a-trust-bundle-is-one-file-of-certificates.md) against the real binary, and the write is proved with the in-process node.
- A user node cannot be reached over SSH, so phase 1 covers server nodes, which is the set that `apply` covers. `meshy bundle` still writes one authority, and a user node takes the bundle on its next `meshy bundle`. The rule of this ticket forbids a change to `meshy bundle`.
- A server node that the vault holds no certificate for is named and the run exits 1. Phase 1 signs no certificate, so it cannot repair that node, and a silent skip would read as a bundle that the node holds.
- Each refusal line carries the `meshy ca rotate: ` prefix. The message after the prefix is the exact text of the ticket.
- The next name comes from `ca.trust`, not from every name in the vault. A retired authority stays in the vault as a record, and a name that it holds must not skip a number. A full rotation leaves `ca-2` trusted, so the next phase 1 creates `ca-3`.
