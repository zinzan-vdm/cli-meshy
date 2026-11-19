---
# MESHY-035
title: The mesh drops the old authority
status: completed
type: task
priority: normal
tags:
    - certificates
    - rotation
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T23:55:00Z
parent: MESHY-024
blocked_by:
    - MESHY-034
---
## Overview

`meshy ca rotate --phase 3` removes the old authority from `ca.trust` and applies the new bundle to every node. The rotation is finished, and the old key stays in the vault for the record.

## Context / Background

Trusting an authority nobody uses is a standing risk. Phase 3 closes it, and it refuses to run while any node still holds a certificate from the old authority.

The key is kept, not deleted. A deleted key cannot explain a certificate that is still in a backup.

Part of [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the trust anchor rotates without downtime.

## Specification

```
meshy ca rotate --phase 3 [--mesh <dir>]
```

| Step | Work |
|---|---|
| 1 | read the signing authority from every node |
| 2 | remove the old authority from `ca.trust` in `mesh.yml` |
| 3 | apply the new bundle to every node |
| 4 | read back the trust bundle from every node |

| Refusal | Message |
|---|---|
| a node still holds an old certificate | `web-1 still holds a certificate from ca-1` |
| `ca.trust` holds one authority | `the rotation is already complete` |

## Technical Notes

- Phase 3 never deletes `ca-1.key` from the vault. It only leaves `ca.trust`.
- The read back is the proof that no node can still accept the old authority.
- A rotation against two nodes completes with no dropped tunnel.

## Reference Material

- [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the parent epic.
- [MESHY-034](MESHY-034--every-node-holds-a-new-certificate.md): phase 2.
- [MESHY-016](MESHY-016--remove-a-node-from-the-mesh.md): the other command that pushes a blocklist.

## Acceptance Criteria

- [x] Phase 3 removes the old authority from `ca.trust` and applies the new bundle.
- [x] The old key stays in the vault.
- [x] Phase 3 is refused while a node still holds a certificate from the old authority.
- [x] The mesh completes the rotation with no dropped tunnel.
- [x] The command reports that the rotation is complete.

## Summary of Changes

- `cmd/meshy/rotate.go`: `rotatePhase3`, `oldAuthority`, `signingAuthorities`, `trustedFingerprint`, and `rotateRun.drop`, which checks that no bundle keeps the authority that the phase dropped.
- `internal/config/trust.go`: `RemoveTrust`, which drops one name from a flow list or a block list and keeps every other byte of the file.
- `internal/config/trust_test.go`: five tests for the edit.
- `cmd/meshy/rotate_test.go`: six tests for the phase, including the refusal that leaves the file and the nodes untouched.
- `test/e2e/rotate/scenario/main.go`: three new assertions. The phase is refused after phase 1, refused while node-a holds a ca-1 certificate, and then removes ca-1 with a ping in flight.
- `docs/adr/0015-the-old-key-outlives-the-rotation.md`, `docs/patterns/0024-a-rotation-reads-the-signer-first.md`, and the rotation section of `README.md`.

## Verification

The live rotation job is the proof, run `44423734094`, job `live rotation`, 38 s. It printed eleven lines:

```
ok 4: phase 1 gives both nodes the trust bundle of ca-1 and ca-2
ok 5: phase 3 refuses before phase 2 has made ca-2 active
ok 6: phase 2 replaces the certificate of node-a (a0bd5d6b -> 2fb9b65d) and of node-b (ce521548 -> 3978f960)
ok 7: the tunnel lost 0 of 9 replies while phase 2 reloaded both nodes
ok 8: every node signs with ca-2 (839d8794)
ok 9: phase 3 refuses while node-a holds a certificate from ca-1
ok 10: phase 3 removes ca-1 and the tunnel lost 0 of 10 replies
ok 11: both nodes trust ca-2 alone (839d8794) and ca-1.key stays in the vault
```

Step 9 builds the state that a stopped phase 2 leaves. The scenario puts the pair that phase 2 kept back on node-a and puts the current pair back after the refusal. Every value above was read from a container, not assumed.

Local gates on Go 1.27.4: `gofmt -l .` prints nothing, `go build ./...` prints `BUILD_OK`, `go vet ./...` prints `VET_OK`, `go test ./...` passes 24 packages with 0 `--- FAIL`, and `write-tism audit.py` prints `total: 0`.

The first run of the job, `44423257674`, failed at step 5. Phase 3 dropped ca-2, because the phase had no guard for the state that phase 1 leaves. That run is the reason the guard and its two tests exist.
