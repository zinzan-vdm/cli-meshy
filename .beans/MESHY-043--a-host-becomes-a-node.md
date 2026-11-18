---
# MESHY-043
title: A host becomes a node
status: todo
type: epic
priority: high
tags:
    - bootstrap
    - install
created_at: 2026-11-18T01:05:00Z
updated_at: 2026-11-18T01:05:00Z
---

## Overview

Two commands turn an empty host into a node. `meshy bootstrap` prepares the user, the sudoers rule, the directory, and the unit. `meshy install` puts the pinned Nebula release on the host. After both, `meshy apply` gives the host its certificate and its config.

## Context / Background

[MESHY-013](MESHY-013--a-merge-changes-the-mesh.md) makes the nodes follow the repository. It assumes a node that already has the unit and the binary, and it reports a missing unit by naming `meshy bootstrap`. That assumption has to be paid somewhere, and this epic pays it.

This work moved out of MESHY-013. Preparing a host is a different job from applying a mesh. It happens once, no merge drives it, and its risk is in the host.

MESHY-021 was left without a parent when [MESHY-020](MESHY-020--binaries-match-the-pin.md) was scrapped, and it belongs here.

## Specification

The order a person runs, once per host:

```
meshy bootstrap <node> | sudo sh
meshy install --node <node>
meshy apply --node <node>
```

| Step | Effect |
|---|---|
| `bootstrap` | prints an idempotent script: the node user, the sudoers rule, `/etc/nebula/meshy-<mesh>/`, and `nebula@meshy-<mesh>.service` |
| `install` | resolves the pin, verifies the checksum, caches the release, uploads the binary, restarts the unit |
| `apply` | signs a certificate, writes the config, reloads |

Neither command needs the authority key. `bootstrap` never runs its script, and `install` never invents a version.

## Technical Notes

- The unit file is written by the script, so a host that has never held it can be prepared with a command from this repository.
- A second `bootstrap` changes nothing. The script is idempotent, so a repair is the same action as the first run.
- The binary is pushed over the same SSH path as the config, so a node needs no package manager and no outbound network.
- A host with a different unit name is not this mesh. The unit name carries the mesh name, so two meshes can share a host.

## Reference Material

- [MESHY-027](MESHY-027--a-maintainer-prepares-a-new-node.md): the host script.
- [MESHY-021](MESHY-021--the-nodes-run-the-pinned-release.md): the pinned binary.
- [MESHY-018](MESHY-018--a-person-joins-with-one-archive.md): the user device case, which is a bundle rather than a unit.
- [ADR 0003](../docs/adr/0003-the-repository-is-the-mesh.md): the repository is the mesh.

## Status

All three commands exist: `bootstrap` in MESHY-027, `install` in MESHY-021, and `apply` in MESHY-014. The criterion below stays open until a run on an empty host is recorded, and the harness that records it is MESHY-030.

## Acceptance Criteria

- [ ] An empty host runs the mesh after `bootstrap`, `install`, and `apply`, in that order.
- [ ] A second `bootstrap` on the same host changes nothing.
- [ ] `bootstrap` refuses a node that is not in `mesh.yml`.
- [ ] `install` refuses a release whose checksum does not match.
- [ ] A node that already runs the pinned version is not restarted.
- [ ] Neither command needs the authority key.
