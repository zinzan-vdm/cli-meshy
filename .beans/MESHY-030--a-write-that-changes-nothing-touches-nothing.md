---
# MESHY-030
title: A write that changes nothing touches nothing
status: completed
type: task
priority: high
tags:
    - tests
    - ssh
created_at: 2026-11-17T20:41:41Z
updated_at: 2026-11-18T11:30:21Z
parent: MESHY-028
---

## Overview

An in-process SSH server with an in-memory filesystem lets the read and write paths be tested with no host. It proves the parts that matter: only changed files are written, the rename is atomic, the reload runs once, and a second run does nothing.

## Context / Background

The apply path is where a bug breaks a node. A real host is the wrong test bed for it, because the interesting cases are failures. A write that fails halfway, a unit that is missing, and a second run that must be silent are all easy to stage in memory. On a server they are painful.

Part of [MESHY-028](MESHY-028--tests-prove-the-claims.md): tests prove the claims.

## Specification

Harness:

| Piece | Behaviour |
|---|---|
| server | `golang.org/x/crypto/ssh` with a host key from a fixture |
| auth | one public key from a fixture, no password |
| filesystem | a map from path to contents and mode |
| command log | every command in order, with its exit status |
| failures | a path or a command can be told to fail once |

Cases:

| Case | Assertion |
|---|---|
| first apply | three files written through a temporary name and a rename |
| second apply | no write, no reload |
| one changed rule | one write, one reload |
| reload fails | the write stays, the error names the unit, exit is non-zero |
| unit missing | the error names `meshy bootstrap` |
| read of a node with no certificate | a message that says so |

## Technical Notes

- The harness lives in `internal/sshx/sshtest` so several packages can use it.
- The filesystem records the order of operations, which is how the test proves the rename came last.
- The harness never binds a TCP port. It hands the client an `net.Conn` pair, so it works with no network stack.
- Timeouts are one second, so a hang fails instead of blocking CI.

## Reference Material

- [MESHY-028](MESHY-028--tests-prove-the-claims.md): the parent epic.
- [MESHY-011](MESHY-011--a-maintainer-sees-what-a-node-runs.md): the read path.
- [MESHY-014](MESHY-014--apply-the-config-to-a-node.md): the write path.
- [docs/patterns/0001](../docs/patterns/0001-tests-need-no-root-or-network.md): no root, no network.

## Acceptance Criteria

- [x] The harness serves SSH over an in-memory connection with no listening port.
- [x] Six cases from the table pass.
- [x] The write order is asserted, with the rename last.
- [x] A failed write leaves the previous contents in place.
- [x] A second apply produces no write and no reload.
- [x] The package tests run in under two seconds.
- [x] No test opens a socket.


## Summary of Changes

- `internal/sshx/sshtest/sshtest.go`: an SSH server with a filesystem in memory. It answers the commands that the read and write paths send. It logs every operation and every command with an order number. A test can stage a failure on one path or on one command.
- `internal/sshx/sshtest/client.go`: a client on the in-memory connection. Run, Write, and Read match the session that `internal/apply` takes. Each command has a one second bound, so a hang fails the test.
- `internal/sshx/sshtest/pipe.go`: a pipe with a buffer. `net.Pipe` has no buffer, and the SSH handshake writes from both ends at once.
- `internal/sshx/sshtest/sshtest_test.go`: tests for the harness. They cover a file round trip and a refused key. They check the mode of a write and the order of a write and a rename. They cover the three fault modes.
- `internal/apply/apply_test.go`: the hand-written fake is gone. The twelve cases run through the harness.
- `internal/sshx/read_test.go`: one test reads a node with no certificate through the harness.
- `docs/patterns/0001-tests-need-no-root-or-network.md`: the enforcement section names the harness.

## Deviations

- The case table says three files. A run writes five: blocklist.txt, config.yml, ca.crt, `<node>.crt`, and `<node>.key`. The test asserts all five, in that order.
- The reload case asserts the error. `apply.Run` returns an error, and `cmd/meshy` turns it into a non-zero exit. There is no exit status inside `internal/apply` to assert.
- Three tests in `internal/sshx` still bind 127.0.0.1, because `sshx.Dial` takes a TCP address and this ticket changes no production file. The harness and every test in `internal/apply` open no socket.
- The harness client repeats the write and read command text that `internal/sshx` builds in unexported functions. `internal/sshx` asserts that text in its own tests.
