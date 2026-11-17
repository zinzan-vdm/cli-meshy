---
# MESHY-036
title: The docs follow the doc skills
status: completed
type: task
priority: normal
tags:
    - mesh
    - nebula
created_at: 2026-11-17T21:05:00Z
updated_at: 2026-11-17T21:05:00Z
---

## Overview

Every document in the repository matches the format, the location, and the frontmatter that its skill defines. A reader who knows the doc skills can find a decision, a rule, a question, and a config field where the skills say they live.

## Context / Background

The docs were written from the skills but drifted in four ways. The config reference sat in `docs/` root as a hand-written table. One ADR recorded two decisions. The glossary was missing two terms that the ADRs tag. No `docs/conf/TAGS` existed.

## Specification

| Document | Skill | Location and format |
|---|---|---|
| Glossary | document-domains | `CONTEXT.md`, bold term, definition, `_Avoid_` line |
| Decision | document-domains | `docs/adr/NNNN-slug.md`, frontmatter tags, one decision |
| Rule | document-patterns | `docs/patterns/NNNN-slug.md`, frontmatter tags, scope in the first sentence |
| Question | document-questions | `docs/questions/NNNN-slug.md`, title, status, created, tags |
| Config | document-config | `docs/conf/NNNN-slug.md`, one block per variable with type, required, source, description |

Every directory with numbered documents holds a `TAGS` file, and every tag in a document appears in that file.

## Technical Notes

- The config reference moved from `docs/mesh-yml.md` to `docs/conf/0001-mesh-yml.md` and became one block per variable, named by dotted path.
- ADR 0006 recorded two decisions. It is now 0006, overlay addresses are explicit, and 0007, the nodes hold the state.
- `Address` and `Review` joined the glossary, because the ADRs tag them.
- `docs/conf/TAGS` is new. `docs/adr/TAGS` gained `address` and `plan`.

## Reference Material

- `document-domains`: the glossary and ADR formats.
- `document-patterns`: the pattern format.
- `document-questions`: the question format.
- `document-config`: the config format.
- [MESHY-001](MESHY-001--describe-and-validate-a-mesh.md): the mesh file this config document describes.

## Acceptance Criteria

- [x] `CONTEXT.md` holds only terms, each with a definition and an `_Avoid_` line.
- [x] Every ADR records one decision and carries frontmatter tags.
- [x] Every pattern states its scope in the first sentence and carries frontmatter tags.
- [x] The question file carries a title, a status, a created date, and tags.
- [x] Config documentation lives in `docs/conf/`, one block per variable.
- [x] Every document tag appears in the `TAGS` file of its directory.
- [x] Every numbered directory has contiguous numbering from 0001.
- [x] No document links to a file that does not exist.

## Summary of Changes

- Added `docs/conf/0001-mesh-yml.md` and `docs/conf/TAGS`.
- Removed `docs/mesh-yml.md`.
- Split ADR 0006 into `0006-overlay-addresses-are-explicit.md` and `0007-the-nodes-hold-the-state.md`.
- Added the terms `Address` and `Review` to `CONTEXT.md`.
- Added `address` and `plan` to `docs/adr/TAGS`.
- Updated the README tree and every link that pointed at the old paths.
