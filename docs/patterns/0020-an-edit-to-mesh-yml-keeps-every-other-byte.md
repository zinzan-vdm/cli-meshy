---
tags:
  - config
  - comments
---

# A command edits mesh.yml in place

A command that changes `mesh.yml` rewrites the bytes of the entry that changes and no other byte. It never writes the file back through a YAML encoder.

`mesh.yml` is the file that a person reviews, so a comment, a blank line, and the order of the keys are part of the change. An encoder keeps the comments that hang on a node. It drops the blank lines between the blocks, and such a diff hides the one line that matters.

`config.AddTrust` is the pattern. It finds the `ca` block, then the `trust` key in it. It edits the flow list or the block list that the key holds. A form that it cannot read is an error, not a rewrite.

## Enforcement

`TestAddTrustChangesOneLineAndKeepsTheRest` compares the whole file with the input plus one entry.

`TestAddTrustKeepsEveryCharacterThatItDidNotEdit` asserts that each comment and each blank line survives.

## See also

- [0003](0003-comments-explain-what-the-code-cannot.md): comments explain what the code cannot.
