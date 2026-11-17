---
# MESHY-033
title: The ticket set follows the standard
status: completed
type: task
priority: normal
tags:
    - process
    - tickets
created_at: 2026-11-17T20:31:31Z
updated_at: 2026-11-17T20:54:22Z
---
## Overview

Every ticket in `tickets/` matches the field set, the section order, the naming rules, and the sizing rules in the ticket skills. A reader who knows the skills can navigate the set without a guide.

## Context / Background

The set was written from the skills but drifted in five ways. A hand-written index repeated the frontmatter. Six task titles carried two deliverables. Two commits had no `Ticket:` trailer. Four tasks were named after a technical layer instead of a result. Two epics held a single task.

Each one breaks a rule the skills state plainly. The fixes are listed below, so a future reader can see why a ticket was merged or scrapped.

## Specification

| Rule | Source | Check |
|---|---|---|
| Fields `id`, `title`, `type`, `status`, `priority`, `tags` | use-tickets | present on every ticket |
| `parent` on tasks | use-tickets | present when the task belongs to an epic |
| Six sections in order | author-tickets | headings match the template |
| Summary of Changes when `completed` | use-tickets | present on every completed ticket |
| Reasons for Scrapping when `scrapped` | use-tickets | present on every scrapped ticket |
| No `and` in a task title | author-tickets | one deliverable per title |
| 3 to 10 acceptance criteria | author-tickets | counted per task |
| `Ticket: {id}` in the commit body | use-tickets | present on every implementation commit |
| A task name states a result, not a layer | author-tickets | no `Render`, `Expand`, `Install` in a title |
| A task means something alone | author-tickets | no task that waits on another to matter |
| An epic holds more than one task | author-tickets | no `todo` epic with a single task |

## Technical Notes

- Titles that carried two deliverables were renamed where the deliverable was one, and split where it was two.
- Two tasks were merged into the task they served, because neither meant anything alone.
- Two epics held one task each, and a single task is not an epic.
- File names carry the title slug, so a rename moves the file and every link to it.

## Reference Material

- `use-tickets`: fields, sections, the commit trailer, and the scrapping rule.
- `author-tickets`: the litmus test, the right-size checklist, and the template.
- [MESHY-023](MESHY-023--a-pull-request-plans-the-mesh.md), [MESHY-032](MESHY-032--a-merge-applies-the-mesh.md): the split.
- [MESHY-024](MESHY-024--the-trust-anchor-rotates.md): the epic that gained two tasks.

## Acceptance Criteria

- [x] Every ticket carries the six metadata fields.
- [x] Every ticket has the six sections in the template order.
- [x] Every completed ticket has a Summary of Changes, and every scrapped ticket has its reasons.
- [x] No task title contains `and`.
- [x] No task title names a technical layer instead of a result.
- [x] Every task has between 3 and 10 acceptance criteria.
- [x] Every task that belongs to an epic names it, and no epic names a parent.
- [x] Every epic that is `todo` holds at least two tasks, and a draft epic holds none.
- [x] Every implementation commit on branch `009` carries a `Ticket:` trailer.
- [x] Every link, `parent`, and `blocked_by` resolves.
- [x] Every ticket is a bean in `.beans/`, and `beans check` passes.

## Summary of Changes

- Deleted `tickets/README.md`, the hand-written index.
- Merged MESHY-010 into MESHY-009: a config without a firewall is not a config.
- Merged MESHY-019 into MESHY-018: an archive nobody can install does not join a mesh.
- Scrapped epic MESHY-017 and epic MESHY-020, each of which held one task.
- Split MESHY-025 into three tasks, one per rotation phase: 025, 034, 035.
- Split MESHY-023 into a plan task and MESHY-032, the apply on merge.
- Renamed thirteen tickets so the title states a result: 002, 003, 006, 007, 009, 011, 012, 018, 021, 027, 029, 030, 031.
- Renamed epic MESHY-022 to `CI runs the mesh`.
- Added `depends_on` to 012, 014, 018, 023, 025, 032, 034, 035.
- Added `Ticket:` trailers to the two commits on branch `009` that lacked them.
- Moved all 35 tickets into `.beans/` as beans, keeping the MESHY-NNN ids.
- Removed `tickets/`, because `beans list` is the index and it is generated from the files.
