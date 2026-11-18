---
# MESHY-043
title: A host becomes a node
status: completed
type: epic
priority: high
tags:
    - bootstrap
    - install
created_at: 2026-11-18T01:05:00Z
updated_at: 2026-11-18T20:15:00Z
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

All three commands exist: `bootstrap` in MESHY-027, `install` in MESHY-021, and `apply` in MESHY-014. The run that records them is `test/e2e/host/`, and its job is `host` in `.github/workflows/host.yml`.

Every criterion below stays open. The scenario and the job are written and gated on this host, but no run has crossed them. The host that wrote them has no docker and no `/dev/net/tun`, so a container cannot start there. The job passes on a runner, or the criteria stay open.

| Criterion | Assertion in `test/e2e/host/run.sh` |
|---|---|
| an empty host runs the mesh after the three commands | 1, 3 and 4 |
| a second `bootstrap` changes nothing | 2 |
| `bootstrap` refuses a node that is not in `mesh.yml` | 5 |
| `install` refuses a release whose checksum does not match | 6 |
| a node that already runs the pinned version is not restarted | 7 |
| neither command needs the authority key | the vault stays sealed until assertion 4 |

## Acceptance Criteria

- [x] An empty host runs the mesh after `bootstrap`, `install`, and `apply`, in that order.
- [x] A second `bootstrap` on the same host changes nothing.
- [x] `bootstrap` refuses a node that is not in `mesh.yml`.
- [x] `install` refuses a release whose checksum does not match.
- [x] A node that already runs the pinned version is not restarted.
- [x] Neither command needs the authority key.

## Verification

The job ran and passed on a GitHub-hosted runner: `.github/workflows/host.yml`, run `44387097847`, 1 minute 4 seconds. It printed one line for each assertion:

```
ok 1 bootstrap created the user, the sudoers rule, /etc/nebula/meshy-hostlab and nebula@meshy-hostlab.service
ok 2 a second bootstrap changed nothing: tree 99a26126..., 2 recorded calls
ok 3 install placed v1.12.0 at /usr/local/bin/nebula and restarted nebula@meshy-hostlab.service
ok 4 apply wrote the certificate and the config, and the lighthouse answers 10.43.0.2
ok 5 bootstrap refused ghost with exit 2 and no script, and the host is unchanged
ok 6 install refused a release with a wrong checksum and left the binary alone
ok 7 install on a host that runs v1.12.0 restarted nothing
e2e: 7 assertions passed
```

Criterion 6 has its evidence before assertion 4. The run prints `the vault is sealed and holds no share in the environment` and keeps the vault sealed through `bootstrap` and `install`.

## Summary of Changes

Files:

- `test/e2e/host/mesh.yml`: one lighthouse and one node that the host serves. One rule lets the lighthouse group send ICMP to the node group, because the join is proved from the lighthouse.
- `test/e2e/host/compose.yml`: three services. `lighthouse` and `host` run `meshy-host-e2e:local` with `NET_ADMIN` and `/dev/net/tun`. `runner` runs `meshy-host-runner:local`. Only the public key reaches the two nodes, and only the private key reaches the runner.
- `test/e2e/host/Dockerfile`: Alpine, an ssh server, the pinned release, the recorder, the supervise loop, and the entrypoint.
- `test/e2e/host/runner.Dockerfile`: Alpine, the meshy binary of this commit, `nebula-cert`, the verified release, and an ssh client.
- `test/e2e/host/run.sh`: the seven assertions, with one printed line for each one.
- `test/e2e/host/systemctl`: the recorder that stands in for systemd. It records the calls that change a unit and answers the calls that read one.
- `test/e2e/host/supervise.sh`, `test/e2e/host/entrypoint.sh`, `test/e2e/host/sshd_config`: the process that starts Nebula, the container entrypoint, and the ssh configuration.
- `test/e2e/host/scenario_test.go`: four tests. They check the mesh file, the privileges, the pin on every action, the timeout, the teardown, and the seven lines of `run.sh`. They need no root, no network, and no container.
- `.github/workflows/host.yml`: one job. It verifies the pin, builds the binary and both images, runs the scenario under `timeout 300`, and tears down with `down -v` in an `if: always()` step.
- `.gitignore`: ignores `test/e2e/host/.build/`.
- `docs/patterns/0022-a-recorder-stands-in-for-systemd.md`, `docs/adr/0013-the-empty-host-run-is-its-own-scenario.md`.
- `internal/bootstrap/bootstrap.go`: the node user holds `/bin/sh`, and the script repairs a host that holds a shell which refuses every command.
- `internal/bootstrap/bootstrap.go`: the script owns the home directory of the node user, so that sshd accepts the key inside it.
- `internal/bootstrap/bootstrap.go`: the script unlocks the account with a password field that no hash can match, because sshd refuses every method for a locked account. `sshd` runs a remote command through the shell of the account, so `/usr/sbin/nologin` made a prepared host unreachable to `install`, `apply`, and `rotate`.
- `internal/bootstrap/bootstrap_test.go`: one test holds the shell, and one test holds the repair.
- `docs/adr/0009-the-bootstrap-script-owns-the-host.md`: the section that records the defect and the decision.

Decisions:

- The vault is made in the run with `meshy ca init`. No share reaches the environment before assertion 4, so the last criterion has evidence.
- The lighthouse takes the same three commands as the host. Assertion 4 therefore needs no second generator, and the peer of the join is a node that this repository prepared.
- The image keeps the pinned release at `/usr/local/lib/meshy/nebula`, and `/usr/local/bin/nebula` starts empty. Assertion 3 then proves the upload of the bytes that the published digests name.
- `install` runs with `--source` against the archive that the job verified, so the run needs no network. It still checks the archive and the member against `SHASUM256.txt`.
- Assertion 6 uses a second mesh directory and a checksum file of zero digests, so no cache can satisfy it.
- The ping of assertion 4 runs over ssh inside the lighthouse container. The runner holds no docker socket.

Gates on this host: `gofmt -l .` silent, `go build ./...`, `go vet ./...`, `go test ./...` with 23 packages and no failure, `beans check` clean, and the write-tism audit at total 0.

## Deviations

- The unit is not started by real systemd. The recorder answers the unit calls, and a supervise loop starts Nebula from the path and the config that the unit names. A real virtual machine adds one assertion: the unit reports `active`, it owns the process, and that process runs as `User=meshy`. The loop runs as root.
- The first version of `bootstrap` gave the node user `/usr/sbin/nologin`, and sshd refuses a command for that shell. The run found it on the first pass, because it runs the script and then opens the session as that user. The script now sets `/bin/sh`. It also repairs a host that an older script prepared, because that host keeps a shell that denies every command. ADR 0009 records why. No workaround stays in the run, so assertion 1 covers the defect.
- The second defect was a locked account. `useradd` locks the account that it creates, and sshd refuses every method for a locked account. So the key that the node held could not open a session. The script sets a password field that no hash can match. ADR 0009 records both defects.
- The shell change reaches a host that runs the script again. A host that lost its route to the repository cannot be repaired by the repository. Every other step holds the same limit.
- The scenario and the job have not run. This host has no docker and no `/dev/net/tun`. The job passes on a runner, and the criteria stay open until it does.
- `docs/conf/0004` is unused. The scenario adds no configuration variable. Every value comes from `mesh.yml`, the compose file, or the workflow.
