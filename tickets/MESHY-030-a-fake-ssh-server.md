---
id: MESHY-030
title: A fake SSH server for the apply path
type: task
status: todo
priority: high
parent: MESHY-028
tags: [tests, ssh]
---

## Overview

An in-process SSH server with an in-memory filesystem lets the read and write paths be tested with no host. It proves the parts that matter: only changed files are written, the rename is atomic, the reload runs once, and a second run does nothing.

## Context / Background

The apply path is where a bug breaks a node. A real host is the wrong test bed for it, because the interesting cases are failures. A write that fails halfway, a unit that is missing, and a second run that must be silent are all easy to stage in memory. On a server they are painful.

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

- [MESHY-028](MESHY-028-tests-prove-the-claims.md): the parent epic.
- [MESHY-011](MESHY-011-read-live-state-over-ssh.md): the read path.
- [MESHY-014](MESHY-014-write-config-and-reload.md): the write path.
- [docs/patterns/0001](../docs/patterns/0001-tests-need-no-root-or-network.md): no root, no network.

## Acceptance Criteria

- [ ] The harness serves SSH over an in-memory connection with no listening port.
- [ ] Six cases from the table pass.
- [ ] The write order is asserted, with the rename last.
- [ ] A failed write leaves the previous contents in place.
- [ ] A second apply produces no write and no reload.
- [ ] The package tests run in under two seconds.
- [ ] No test opens a socket.
