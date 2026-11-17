---
# MESHY-026
title: Devices that cannot run Nebula reach the mesh
status: draft
type: epic
priority: deferred
tags:
    - proxy
    - mobile
    - oauth
created_at: 2026-11-17T19:22:37Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

A device that cannot run Nebula, such as a phone, reaches selected services through a proxy that authenticates the user and authorizes the route. This epic is not scheduled. It records the shape of the problem and the questions that must be answered first.

## Context / Background

Nebula on iOS and Android exists only inside an application, and no client accepts a self-hosted bundle. Phones are out of scope for the first release. The open question in `docs/questions/0001` tracks the gap.

## Specification

Not specified. The shape under discussion:

| Concern | Open |
|---|---|
| Where the proxy runs | a lighthouse, a dedicated node, or outside the mesh |
| How a user authenticates | OAuth provider, device code flow, or a one time link |
| What a route is | a node and port, a group, or a named service |
| How a session ends | token lifetime, or a revocation that reaches the proxy |

## Technical Notes

The proxy terminates user traffic and forwards it into the mesh, so it becomes a node with broad reach. That makes it the most privileged host in the design, and the authorization model must be settled before any code.

## Reference Material

- [docs/questions/0001](../docs/questions/0001-devices-without-nebula.md): the open question.
- [MESHY-001](MESHY-001--describe-and-validate-a-mesh.md): the mesh model that a route would name.
- [ADR 0001](../docs/adr/0001-nebula-is-the-data-plane.md): the data plane, and the mobile gap it leaves.

## Acceptance Criteria

- [ ] The authorization model is written as an ADR.
- [ ] The open question is answered and closed.
- [ ] The proxy has a place in `mesh.yml`, or a documented reason it sits outside.
