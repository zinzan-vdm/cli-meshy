---
# MESHY-037
title: Every comment earns its place
status: todo
type: task
priority: normal
tags:
    - comments
    - code
created_at: 2026-11-17T21:55:00Z
updated_at: 2026-11-17T21:55:00Z
---

## Overview

The Go code carries comments only where the code cannot speak for itself. A reader sees the aim, the reason, and the trap. Nothing else.

## Context / Background

The code carries 90 comment lines across eleven files. Some state what a name already states. Some are bare labels. A few carry a fact that a reader cannot infer, and those are the ones worth keeping.

[0003](../docs/patterns/0003-comments-explain-what-the-code-cannot.md) sets the rule. This ticket applies it.

## Specification

For every comment, apply the three conditions from the pattern:

| Condition | Action |
|---|---|
| The code does not show its aim or its reason | Keep the comment, and make it say the aim or the reason |
| The purpose is not obvious from the name and the signature | Keep the comment, and state what the function achieves |
| The implementation is counter-intuitive, and the reason is too small for an ADR | Keep the comment, and say why this way |
| None of the three | Delete the comment |

A doc comment on an exported identifier stays, and states what the identifier achieves rather than what it is.

## Technical Notes

- The heaviest files are `internal/config/config.go` (22 comment lines) and `internal/nebula/firewall.go` (21).
- `internal/config/config.go` holds bare labels such as `// Kinds of node.` and `// Schema is the mesh file version this build understands.` The first states nothing a reader needs, and the second restates the name.
- `internal/nebula/firewall.go` holds one comment worth keeping: the ICMP port note, because Nebula warns and ignores the field.
- Test files are out of scope. A test comment that names the case it covers is a label a reader uses.
- A decision belongs in an ADR. A reason that needs a paragraph is not a comment.

## Reference Material

- [0003](../docs/patterns/0003-comments-explain-what-the-code-cannot.md): the rule this ticket applies.
- [0002](../docs/patterns/0002-renderers-are-pure.md): the rule that keeps the renderer testable.
- [MESHY-009](MESHY-009--a-node-gets-the-config-it-must-run.md): the code that holds the ICMP note.

## Acceptance Criteria

- [ ] No comment restates the statement below it.
- [ ] Every comment that remains satisfies one of the three conditions.
- [ ] Every exported identifier keeps a doc comment that states what it achieves.
- [ ] No behaviour changes, and `go test ./...` passes with the same tests.
- [ ] `gofmt -l .` and `go vet ./...` report nothing.
