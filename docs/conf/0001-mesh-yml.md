---
tags:
  - config
  - mesh
---

# mesh.yml System Variables

The file that describes one mesh. `meshy` reads it from `<mesh>/mesh.yml`.

## `schema`

- Type: number
- Required: no
- Default: 1
- Source: config file
- Description: The file format version.
- Constraints: A binary refuses a schema it does not know.

## `name`

- Type: string
- Required: yes
- Source: config file
- Description: The mesh name.
- Constraints: Lowercase letters, digits, and dashes. 32 characters at most.

## `cidr`

- Type: string
- Required: yes
- Source: config file
- Description: The overlay network in IPv4.
- Constraints: A network address, such as `10.42.0.0/24`.

## `interface`

- Type: string
- Required: no
- Default: generated from the mesh name
- Source: config file
- Description: The TUN device name on every node.
- Constraints: 15 characters at most.

## `nebula.version`

- Type: string
- Required: no
- Default: `v1.12.0`
- Source: config file
- Description: The release that every server node runs.
- Constraints: A release tag, such as `v1.12.0`. `meshy install` fetches this release. `apply` never changes a binary.

## `ca.duration`

- Type: duration
- Required: no
- Default: 20y
- Source: config file
- Description: The lifetime of a new certificate authority.
- Constraints: `s`, `m`, `h`, `d`, and `y`. One year is 365 days. Must outlast the longest certificate lifetime.

## `ca.rotate_at`

- Type: percentage
- Required: no
- Default: 80%
- Source: config file
- Description: The age at which `plan` proposes a rotation.
- Constraints: A percentage of `ca.duration`. The tool never rotates by itself.

## `ca.trust`

- Type: list of strings
- Required: no
- Default: `[ca-1]`
- Source: config file
- Description: The authorities that every node trusts. `meshy ca init` writes this when the file does not name it.
- Constraints: Each name must exist in the vault. A node accepts several at once.

## `ca.active`

- Type: string
- Required: no
- Default: `ca-1`
- Source: config file
- Description: The authority that signs new certificates. `meshy ca init` writes this when the file does not name it.
- Constraints: Must appear in `ca.trust`.

## `lighthouses`

- Type: list of objects
- Required: yes
- Source: config file
- Description: The nodes that answer peer discovery.
- Constraints: At least one entry. Each entry holds `node` and `port`. The node must be `kind: server` and must set `public_addr`.

## `certificates.server`

- Type: duration
- Required: no
- Default: 1y
- Source: config file
- Description: The lifetime of a certificate for a server node.
- Constraints: Longer than `certificates.renew_before`.

## `certificates.user`

- Type: duration
- Required: no
- Default: 1y
- Source: config file
- Description: The lifetime of a certificate for a user node.
- Constraints: Longer than `certificates.renew_before`.

## `certificates.renew_before`

- Type: duration
- Required: no
- Default: 60d
- Source: config file
- Description: The remaining life at which `apply` renews a certificate.
- Constraints: Shorter than the shortest certificate lifetime.

## `nodes`

- Type: list of objects
- Required: yes
- Source: config file
- Description: The hosts in the mesh.
- Constraints: Each entry holds `name`, `kind`, `groups`, and `ip`. A `server` node holds `ssh` and no `platform`. A `user` node holds `platform` and no `ssh`.

Each `ip` is written down, never allocated. It must sit inside `cidr` and must not be the network or broadcast address.

`public_addr` is the underlay address of a lighthouse, in `host:port` form.

## `rules`

- Type: list of objects
- Required: no
- Source: config file
- Description: The access rules that the destinations enforce.
- Constraints: Each entry holds `from`, `to`, and `ports`. A source or destination is a node name, a group name, or `any`. A port is `port/proto`, `first-last/proto`, or `any/icmp`.

Deny is the default and cannot be written. A rule from a group that no node carries is an error, because it grants nothing.

## Validation

`meshy validate` reports every problem at once, and the same checks run before `plan` and `apply`. A file that fails validation is never applied.
