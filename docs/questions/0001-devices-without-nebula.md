---
title: How do devices that cannot run Nebula reach the mesh?
status: open
created: 2026-11-17
tags:
  - proxy
  - mobile
  - oauth
---

## Question

Phones cannot run Nebula without an app. What is the design for a device that cannot join the mesh directly?

## Answer

## Discussion

2026-11-17 (Horo): A future proxy instance could accept public traffic and authenticate users with an OAuth provider such as Google. It would then authorize which routes the user can reach. The design is open and may be complex.
