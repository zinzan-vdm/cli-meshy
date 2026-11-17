---
tags:
  - comments
  - code
---

# Comments explain what the code cannot

Every Go file in this repository carries no comments, unless one of three conditions holds.

## The three conditions

1. **The code does not show what it aims to do, or why it does it that way.** Write the aim. Write the reason a reader cannot infer.
2. **The purpose of the code is not obvious from the name and the signature.** Write what the function achieves.
3. **The implementation is counter-intuitive, and the reason is too small for a domain document.** Write why this way and not the obvious way.

## What is not a comment

A comment that restates the code is a defect. So is a comment that repeats a name, a type, or a decision that an ADR already records.

```go
// NumberPort returns a single port.
// The name already says this, so the comment is a defect.
func NumberPort(n int) Port

// Nebula warns when an ICMP rule carries a port, so the field is left out.
// A reader cannot infer this, so the comment stays.
func (p Port) IsZero() bool
```

## Enforcement

A review rejects a comment that restates the code. A review also rejects a block whose aim a reader cannot see, because that block needs a comment.
