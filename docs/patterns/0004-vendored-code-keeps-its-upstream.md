---
tags:
  - vendoring
  - licence
---

# Vendored code keeps its upstream

Code we copy from another project stays traceable to it.

## The rules

1. **One package per upstream project.** The code sits under `internal/`, in a package named after the upstream one.
2. **The upstream notices stay.** The copyright line and the `SPDX-License-Identifier` line are copied unchanged.
3. **A header names the source.** Above the notices: the upstream path, the module version, the licence, and the date the file was taken.
4. **The body stays as upstream wrote it.** An update is a diff against a new version, not a rewrite. A local fix is a comment beside the line and a note in the header, so the next diff shows both.
5. **Our tests state what we rely on.** The upstream test file is not copied. Our tests cover the behaviour this repository needs, and they fail if an update breaks it.
6. **The README credits the upstream.** One line under Licence, with the path a reader needs to check the source.

## The licence check

The upstream licence must be compatible with GPL-3.0. MPL-2.0 is, because the file-level licence governs the file and the MPL grants the GPL-3.0 permission. A licence that forbids redistribution is not, and the code is not copied at all.

## Why vendor

A module that supplies one small function can bring hundreds of modules with it. `internal/shamir` replaced a dependency that brought 514, and another that had no release since 2017. One file of 6.5 KB was the whole of what the vault needed.

## What this is not

This is not a place to fork a library and grow it. Vendored code is frozen until someone diffs it against a new upstream version. New behaviour goes in our own packages, around the vendored one.
