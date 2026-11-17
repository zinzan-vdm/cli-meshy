---
id: MESHY-033
title: The ticket set follows the standard
type: task
status: completed
priority: normal
tags: [process, tickets]
---

## Overview

Every ticket in `tickets/` matches the field set, the section order, and the naming rules in the ticket skills. A reader who knows the skills can navigate the set without a guide.

## Context / Background

The set drifted from the skills in three places. A hand-written index file repeated the frontmatter. Six task titles carried two deliverables. Two commits had no `Ticket:` trailer. Each one breaks a rule the skills state plainly.

The index was removed because it repeated frontmatter that already exists and went stale twice during renumbering.

## Specification

| Rule | Source | Check |
|---|---|---|
| Fields `id`, `title`, `type`, `status`, `priority`, `tags` | use-tickets | present on every ticket |
| `parent` on tasks | use-tickets | present when the task belongs to an epic |
| Six sections in order | author-tickets | headings match the template |
| Summary of Changes when `completed` | use-tickets | present on every completed ticket |
| No `and` in a task title | author-tickets | one deliverable per title |
| 3 to 10 acceptance criteria | author-tickets | counted per task |
| `Ticket: {id}` in the commit body | use-tickets | present on every implementation commit |

## Technical Notes

- Titles that carried two deliverables were renamed where the deliverable was one, and split where it was two.
- MESHY-023 lost its apply half to MESHY-032, because a plan on a pull request and an apply on a merge are separate workflows.
- File names carry the title slug, so a rename moves the file and every link to it.

## Reference Material

- `use-tickets`: fields, sections, and the commit trailer.
- `author-tickets`: the litmus test, the right-size checklist, and the template.
- [MESHY-023](MESHY-023-a-pull-request-plans-the-mesh.md), [MESHY-032](MESHY-032-a-merge-applies-the-mesh.md): the split.

## Acceptance Criteria

- [x] Every ticket carries the six metadata fields.
- [x] Every task that belongs to an epic names it, and no epic names a parent.
- [x] Every ticket has the six sections in the template order.
- [x] No task title contains `and`.
- [x] Every task has between 3 and 10 acceptance criteria.
- [x] Every completed ticket has a Summary of Changes.
- [x] Every implementation commit on this branch carries a `Ticket:` trailer.

## Summary of Changes

- Deleted `tickets/README.md`, the hand-written index.
- Renamed five ticket files and titles: 005, 014, 016, 023, 029.
- Renamed the title of MESHY-012 without moving its file.
- Added `tickets/MESHY-032-a-merge-applies-the-mesh.md`.
- Added `depends_on` to MESHY-012, MESHY-014, MESHY-023, and MESHY-032.
- Added `Ticket:` trailers to the two commits on branch `009` that lacked them.
