---
# MESHY-046
title: The plan reports when the authority is due for rotation
status: todo
type: task
priority: normal
tags:
    - certificates
    - rotation
    - reporting
created_at: 2026-11-18T22:44:43Z
updated_at: 2026-11-18T22:44:43Z
parent: MESHY-024
blocked_by:
    - MESHY-035
---

## Overview

`meshy plan` reads the active authority from a node trust bundle. It prints the age of that authority and the next rotation phase. It prints a warning when the authority reaches `ca.rotate_at`. It never fails a pull request for a due rotation.

## Context / Background

`ca.rotate_at` is documented as the age at which `plan` proposes a rotation. Nothing reads that value today. The plan compares each node trust bundle with `ca.trust`, so it never reports how old the anchor is.

Without this report, a rotation depends on a calendar that nobody checks. The epic cannot close until the plan states the age.

The plan runs with a closed vault on a pull request from a fork, and a fork holds no secrets. The node trust bundle is the only source that works on every path. It is also the source that matters, because a rotation concerns what the nodes trust.

Part of [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the trust anchor rotates without downtime.

## Specification

### Text

`meshy plan` prints this block above the node rows when at least one bundle parses.

```
mesh hostlab
  authority  ca-1 (0% of 20y)
  due        no
  next       phase 1 from 2045-11-18
```

When the authority reaches the threshold:

```
mesh hostlab
  authority  ca-1 (81% of 20y)
  due        yes
  next       phase 1 now: meshy ca rotate --phase 1
```

### JSON

`meshy plan --json` adds one top-level object. The object is absent when no authority is read.

```json
"rotation": {
  "authority": "ca-1",
  "not_before": "2026-11-18T14:37:52Z",
  "not_after": "2046-11-18T14:37:52Z",
  "used": 0.4,
  "rotate_at": 80,
  "due": false,
  "next": "phase 1"
}
```

| Field | Type | Meaning |
|---|---|---|
| `authority` | string | the name of the active authority, from `ca.active` |
| `not_before` | string | the start of the authority lifetime, RFC 3339 |
| `not_after` | string | the end of the authority lifetime, RFC 3339 |
| `used` | number | the age as a percentage of the lifetime, one decimal |
| `rotate_at` | number | the configured threshold, as a percentage |
| `due` | boolean | true when `used` reaches `rotate_at` |
| `next` | string | `phase 1`, `phase 2`, or `phase 3` |

### The next phase

| Trust count | Node certificates | `next` |
|---|---|---|
| 1 | no node holds a certificate from another authority | `phase 1` |
| 2 | a node still holds a certificate from the old authority | `phase 2` |
| 2 | every node holds a certificate from the new authority | `phase 3` |

### Exit code

The plan keeps three outcomes. A match returns 0. A difference returns 1. A plan that could not be computed returns 2. A due rotation changes none of them.

## Technical Notes

- `internal/node/state.go`: add `NotBefore string` to `Cert`, tagged `json:"notBefore"`. The field already appears in the fixtures.
- `internal/plan/plan.go`: add `Rotation *Rotation` to `Report`, tagged `json:"rotation,omitempty"`. Take the authority name from `mesh.CA.Active`, and the dates from the bundle entry with that name.
- `used` is the age divided by the lifetime, as a percentage with one decimal. `Input.Now` supplies the clock, so a test needs no wall clock.
- `next` derives from `len(mesh.CA.Trust)` and each readable `Cert.Issuer`. Do not add a state field.
- `cmd/meshy/plan.go`: print the block from `formatText` before the node rows. The node rows keep the current `name kind detail` shape.
- `internal/scaffold/scaffold.go`: in both plan steps, print a `::warning` annotation when the plan reports a due rotation. Read the field with `grep -q '"due": true'`. Do not use `jq`.
- `internal/scaffold/workflow_test.go`: assert the annotation, and assert that `jq` does not appear.
- Tests cover 79.9 and 80.0 percent, a two-authority mesh in phase 2 and in phase 3, and a mesh with no readable node.
- Documents: ADR 0014 "A due rotation is information, not drift", and pattern 0023 "The plan reads the authority from the bundle". `docs/conf/0001-mesh-yml.md` already describes `rotate_at` as this report. Change that entry only if the implementation makes it false.

## Reference Material

- [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the parent epic.
- [MESHY-034](MESHY-034--every-node-holds-a-new-certificate.md): phase 2 signs from the new authority.
- [MESHY-035](MESHY-035--the-mesh-drops-the-old-authority.md): phase 3 drops the old authority.
- [docs/adr/0011-a-trust-bundle-is-one-file-of-certificates.md](docs/adr/0011-a-trust-bundle-is-one-file-of-certificates.md): one file holds every trusted authority.
- [docs/patterns/0008-a-difference-is-a-report.md](docs/patterns/0008-a-difference-is-a-report.md): the plan is a report and a workflow parses it.
- [docs/adr/0002-github-actions-is-the-primary-executor.md](docs/adr/0002-github-actions-is-the-primary-executor.md): the workflow is the primary executor.
- [docs/conf/0001-mesh-yml.md](docs/conf/0001-mesh-yml.md): `ca.rotate_at` and `ca.duration`.

## Acceptance Criteria

- [ ] `meshy plan` prints the three-line block above the node rows when a bundle parses.
- [ ] `meshy plan --json` carries the `rotation` object with every field in the specification.
- [ ] The plan omits `rotation` when no node bundle parses, and still reports the node error.
- [ ] `used` has one decimal, and `due` is true when `used` reaches `rotate_at`.
- [ ] A mesh with two trusted authorities reports `phase 2` while a node holds a certificate from the old authority.
- [ ] A mesh with two trusted authorities reports `phase 3` when every node holds a certificate from the new authority.
- [ ] A due rotation leaves the exit code at 0 when every node matches, and the JSON reports `due` true.
- [ ] Both plan steps in the generated workflow print a warning annotation when the plan reports a due rotation.
- [ ] `internal/scaffold/workflow_test.go` fails when `jq` appears in the generated workflow.
