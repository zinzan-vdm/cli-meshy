---
tags:
  - nebula
  - rendering
  - revocation
---

# The blocklist is a file that the config names

In `internal/nebula`, render one fixed blocklist path in `pki.blocklist`. Never render the fingerprints into the config.

## Rationale

`plan` compares the hash of the config that it renders with the hash that a node reports. A blocklist inside the config would change that hash, and only a run that can open the vault knows the fingerprints. A pull request from a fork cannot open the vault, so the plan would report a difference that it cannot explain. It would also report that difference again on every later run.

The vault holds the fingerprints. The file carries them, and its path is a constant.

## Enforcement

`TestComputeKeepsTheBlocklistOutOfTheConfigComparison` asserts that a plan with a revocation holds no config change for a node that matches. The golden files in `internal/nebula/testdata` carry the fixed path.

## See also

- [0014](0014-a-block-outlives-the-certificate.md): the fingerprint that the file carries.
- [0002](0002-renderers-are-pure.md): the renderer takes values and returns bytes.
