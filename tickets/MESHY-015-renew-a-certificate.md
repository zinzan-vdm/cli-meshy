---
id: MESHY-015
title: Renew a certificate inside the window
type: task
status: todo
priority: normal
parent: MESHY-013
tags: [certificates, apply]
---

## Overview

`apply` renews any certificate that is inside `certificates.renew_before` of expiry, writes it to the node, and reloads. Renewal keeps the same address and the same groups, so no peer notices.

## Context / Background

Certificates last one year by default, and a repository may go months without a merge. Renewal on every apply, plus a weekly scheduled check, is the mechanism that keeps the mesh alive without a calendar reminder.

## Specification

| Condition | Action |
|---|---|
| certificate missing on the node | issue, write, reload |
| remaining life at or below `renew_before` | issue, write, reload |
| certificate in the vault differs from the node | write the vault copy, reload |
| remaining life above the window | nothing |

Renewal uses the authority named by `ca.active`. A user node is never renewed remotely. Its bundle is rebuilt and the maintainer hands it over.

## Technical Notes

- Issuance is the same code path as `meshy issue`, so the two cannot drift.
- The new certificate keeps the address, which is what allows a reload instead of a restart.
- A renewal inside a run that also changes rules produces one reload, not two.

## Reference Material

- [MESHY-007](MESHY-007-issue-a-certificate.md): the issuance path.
- [MESHY-013](MESHY-013-a-merge-changes-the-mesh.md): the parent epic.
- [docs/mesh-yml.md](../docs/mesh-yml.md): the `certificates` block.

## Acceptance Criteria

- [ ] A certificate inside the window is replaced and the node reloads.
- [ ] A certificate outside the window is left alone.
- [ ] The renewed certificate keeps the address and the groups.
- [ ] A user node is never renewed over SSH.
- [ ] A rule change and a renewal in one run cause one reload.
- [ ] The plan reports the same renewal before it happens.
