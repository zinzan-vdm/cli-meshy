---
tags:
  - bootstrap
  - install
  - systemd
---

# The empty-host run is its own scenario and its own job

The run that takes a host from empty to a node lives in `test/e2e/host/`, with its own compose file and its own workflow, `.github/workflows/host.yml`.

## Why not the live mesh job

The live mesh job in `test/e2e/` proves the firewall and the reload on a mesh that is already up. Its nodes start with their configs on a volume, and it never runs `bootstrap` or `install`. The empty-host run starts with no `/etc/nebula` content and proves the three commands in order. A failure in one job reads as a failure of the other when the two share a file.

The two runs also take different inputs. The live mesh renders every config before the containers start. The empty-host run needs a sealed vault, three shares, and a binary that arrives over SSH during the run.

## Why a second compose file

The compose file names three services, and two of them run one image. Sharing the file would mean a profile, a variable, or a branch in the workflow. A copy that a reader holds in one screen is cheaper than a shared file with two modes.

## Consequences

The two scenarios duplicate the download steps, the `timeout`, and the teardown. A change to the release URL lands in two files. A reviewer sees both files in one pull request.

The host image is not the mesh image. The host image holds an ssh server and the recorder that stands in for systemd. The mesh image holds the harness. A change to one cannot break the other.

## See also

- [ADR 0002](0002-github-actions-is-the-primary-executor.md): CI is the executor.
- [pattern 0022](../patterns/0022-a-recorder-stands-in-for-systemd.md): the recorder that the host image holds.
