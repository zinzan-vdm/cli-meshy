---
# MESHY-037
title: Every comment earns its place
status: completed
type: task
priority: normal
tags:
    - comments
    - code
created_at: 2026-11-17T21:55:00Z
updated_at: 2026-11-17T22:40:00Z
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

A doc comment on an exported identifier stays only where the name and the signature do not say what it achieves.

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

- [x] No comment restates the statement below it.
- [x] Every comment that remains satisfies one of the three conditions.
- [x] No exported identifier carries a comment that restates its name, and any comment it keeps states what it achieves.
- [x] No behaviour changes, and `go test ./...` passes with the same tests.
- [x] `gofmt -l .` and `go vet ./...` report nothing.

## Summary of Changes

The code now carries 58 comments instead of 75: 17 deleted, 7 reworded, and 51 left alone because they already satisfied a condition.

### Files

- `internal/config/config.go`: 22 comments to 13. The bare section labels in `Validate` and the doc comments on `Schema`, `CA`, and `Certificates` are gone. `Load` and `GenerateInterface` now state what they achieve.
- `internal/nebula/firewall.go`: 21 to 17. The `Port`, `sourceOf`, and `parsePorts` comments say what the code does. The ICMP note stays, because Nebula warns and ignores the field and a reader cannot infer that.
- `internal/nebula/render.go`, `internal/node/read.go`, `internal/scaffold/scaffold.go`, `internal/sshx/client.go`, `internal/config/duration.go`, `internal/config/helpers.go`: one comment each removed or reworded.
- `docs/patterns/0003-comments-explain-what-the-code-cannot.md`: the rule this ticket applies, with one rule, one scope, and good and bad examples.
- `docs/patterns/TAGS`: `code` and `comments`.

### Decisions

- An exported identifier loses its doc comment when the name and the signature already say what it achieves. Go style asks for a comment on every exported identifier. This pattern asks for one only where the code cannot speak, and the pattern wins.
- The test files are untouched. A test comment that names the case is a label a reader uses.

### Deviations

None. The diff is comment lines only, and the test set is unchanged.
