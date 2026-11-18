---
tags:
  - ci
  - vault
---

# The vault opens only for a change

In the mesh workflow, the `apply` job plans with a closed vault. It unseals the vault only when the plan holds a change or when a merge started the run. The `lock` step runs on `always()`.

A weekly run that finds a match applies nothing, so it does not need the authority key on the runner. A plan reads the certificate of each node over SSH, so it reports drift and a renewal without the vault. A merge is the approval, so it applies the plan that the review saw.

The `lock` step holds `if: always()`, so a failed apply cannot leave the plaintext directory behind. A second `always()` step removes the ssh identity.

## Enforcement

`TestApplyJobAppliesOnlyWhenThePlanChanges` reads the generated workflow and asserts the plan output that gates the `unlock` and `apply` steps. `TestApplyJobClosesTheVaultAndRemovesTheKey` asserts the `always()` condition and the step order.

## See also

- [0008](0008-a-difference-is-a-report.md): a difference is a report.
- [0007](0007-pin-every-action-to-a-commit-sha.md): pin every action to a commit SHA.
