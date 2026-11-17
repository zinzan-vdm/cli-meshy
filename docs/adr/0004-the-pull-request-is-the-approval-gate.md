---
tags:
  - automation
  - review
---

# The pull request is the approval gate

Adding a node mints an identity, so the moment that needs a human is the review of the diff that adds it. We protect `main`, require a review, and put a CODEOWNERS rule on `mesh.yml`. `apply` on merge then runs unattended. We do not use GitHub environment reviewers, because required reviewers on a private repository need GitHub Enterprise.

## Consequences

No workflow job declares an environment, so environment secrets are not part of the design. The vault shares live in repository secrets.

A pull request that adds, removes, or re-groups a node cannot merge without owner review. A pull request that only changes rules still needs review, because `mesh.yml` is a single file.
