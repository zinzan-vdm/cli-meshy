---
tags:
  - mesh
  - nebula
  - transport
---

# Nebula is the data plane, cli-meshy is the control plane

cli-meshy must mesh hosts and enforce `source -> destination:port` access. Nebula already does both: certificates carry identity and groups, lighthouses handle discovery, UDP hole punching crosses NAT, and each node enforces its own inbound rules. We use Nebula for the tunnel, the PKI, and the packet filter. cli-meshy only generates, distributes, and reloads Nebula configuration. We do not build a data plane.

## Considered Options

**WireGuard.** Mature, and its macOS and Windows clients are the easiest path for user machines. But it has no discovery, no hole punching, and no access-control model. We would build the rule compiler, the distribution layer, and nftables enforcement on every destination node, plus a discovery design. That is the work we want to avoid.

**Nebula.** Runs on Linux, macOS, Windows, iOS, and Android. Firewall is default deny with no deny rules, and each rule matches `port`, `proto`, and a source of host, CIDR, or certificate group. Rules are enforced on the destination, which is the model we want. `SIGHUP` reloads firewall and certificates without tearing down tunnels, and connection tracking re-validates each packet against the new ruleset.

## Consequences

User machines need the Nebula binary, a certificate, a key, and a config file. WireGuard users would only import one config. This is the cost we accept.

Revocation is a certificate blocklist. A revoked node stays reachable until every node that trusts it gets the new blocklist. The control plane must push blocklists as a first-class operation.
