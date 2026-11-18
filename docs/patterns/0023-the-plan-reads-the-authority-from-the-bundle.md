---
tags:
  - certificates
  - rotation
---

# The plan reads the authority from the bundle

In `internal/plan`, the age of the active authority comes from the trust bundle that a readable node presents. It never comes from the vault.

The vault is sealed on a pull request, and a pull request from a fork holds no shares. The bundle is the only source that answers on every path. It is also the source that matters, because a rotation concerns what the nodes trust.

A bundle with no dates reports no age. The plan then prints no block, and the node keeps the error or the `unread` mark that it already had. A guessed age is worse than no age.

## Enforcement

`TestComputeReportsNoRotationWithoutABundle` asserts that a bundle with no dates and a node that cannot be read both leave `rotation` absent.

`TestPlanPrintsTheAgeOfTheAuthority` asserts the block from a real plan run against a test server, with no vault.

## See also

- [0008](0008-a-difference-is-a-report.md): a difference is a report.
- [0011](../adr/0011-a-trust-bundle-is-one-file-of-certificates.md): a trust bundle is one file of certificates.
