---
# MESHY-041
title: A new node gets its certificate
status: scrapped
type: task
priority: high
tags:
    - certificates
    - signing
    - apply
created_at: 2026-11-18T00:40:00Z
updated_at: 2026-11-20T19:32:25Z
---

## Reasons for Scrapping

[MESHY-007](MESHY-007--a-node-gets-its-certificate.md) already covers this step. It is `todo`, it is ready, and it belongs to [MESHY-004](MESHY-004--the-vault-seals-and-unseals.md), whose last open criterion is a certificate for a node.

This ticket also made a decision that the repository had already made the other way. It said the node key is made in a temporary directory and never stored. MESHY-007 stores the certificate and the key in the vault, so `issue` can run once and the node can be provisioned later without the authority key.

MESHY-007 keeps the useful parts. It takes the flag names, the issuer check on the printed certificate, and the temporary directory for a key that must not persist.
