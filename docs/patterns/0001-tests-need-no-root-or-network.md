---
tags:
  - tests
  - ci
---

# Tests need no root and no network

Every `_test.go` file in this repository must pass as an unprivileged user with no network access and no container runtime. A test that needs root, a TUN device, or docker carries `//go:build e2e` and runs in its own CI job.

## Rationale

The development host has 954 MB of memory and no docker. A mesh lab on it stopped the agent. The privilege boundary is what keeps `go test ./...` cheap enough to run on every change.

## Examples

Good: the renderer is tested by comparing bytes with a golden file.

Bad: a test starts three Nebula processes and pings across the overlay. That belongs behind the `e2e` tag.

## Enforcement

`go test ./...` in the `ci` job runs with no elevated rights. A test that needs them fails there, which is the signal.
