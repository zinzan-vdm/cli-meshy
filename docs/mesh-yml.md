# mesh.yml

One file describes one mesh. `meshy` reads it from `<mesh>/mesh.yml`.

Ten top level keys. Every value below that has a default can be left out.

## Top level

| Key | Default | Meaning |
|---|---|---|
| `schema` | 1 | File format version. A binary refuses a schema it does not know. |
| `name` | none | Mesh name. Lowercase letters, digits, and dashes. 32 characters at most. |
| `cidr` | none | Overlay network in IPv4. |
| `interface` | generated | TUN device name. 15 characters at most. |
| `nebula` | see below | The pinned data plane. |
| `ca` | see below | The certificate authority. |
| `lighthouses` | none | The nodes that answer peer discovery. |
| `certificates` | see below | Certificate lifetimes. |
| `nodes` | none | The hosts in the mesh. |
| `rules` | none | The access rules. |

## nebula

```yaml
nebula:
  version: v1.12.0
```

`version` is the release that every server node runs and that `meshy install` fetches. `plan` reports a node that runs a different version, and `apply` does not change it. Installing is a separate command.

## ca

```yaml
ca:
  duration: 20y
  rotate_at: 80%
  trust: [ca-1, ca-2]
  active: ca-2
```

| Key | Default | Meaning |
|---|---|---|
| `duration` | 20y | Lifetime of a new certificate authority. |
| `rotate_at` | 80% | Age at which the tool proposes a rotation. `plan` reports it. It never rotates by itself. |
| `trust` | `[ca-1]` | The authorities that every node trusts. A node accepts several at once. |
| `active` | `ca-1` | The authority that signs new certificates. It must appear in `trust`. |

Rotation is three reviewed changes: add the new authority to `trust`, move `active`, then remove the old authority.

## lighthouses

```yaml
lighthouses:
  - node: core-1
    port: 4242
```

At least one is required. Each `node` must exist, must be `kind: server`, and must set `public_addr`. Peers find each other through these nodes, and UDP hole punching lets the rest sit behind NAT.

## certificates

```yaml
certificates:
  server: 1y
  user: 1y
  renew_before: 60d
```

| Key | Default | Meaning |
|---|---|---|
| `server` | 1y | Lifetime of a certificate for a server node. |
| `user` | 1y | Lifetime of a certificate for a user node. |
| `renew_before` | 60d | Remaining life at which `apply` renews. |

Durations accept `s`, `m`, `h`, `d`, and `y`. One year is 365 days.

Two rules are checked: `renew_before` must be shorter than the shortest lifetime, and `ca.duration` must outlast the longest lifetime.

Renewal needs no re-enrolment. A new certificate with the same address takes effect on reload.

## nodes

```yaml
nodes:
  - name: core-1
    kind: server
    groups: [core]
    ip: 10.42.0.1
    ssh:
      user: meshy
      host: 203.0.113.10
      port: 22
    public_addr: 203.0.113.10:4242

  - name: horo-laptop
    kind: user
    groups: [user, admin]
    ip: 10.42.0.50
    platform: darwin/arm64
```

| Key | Meaning |
|---|---|
| `name` | Unique in the mesh. Lowercase letters, digits, and dashes. 32 characters at most. It travels in the certificate. |
| `kind` | `server` is managed over SSH. `user` installs a bundle by hand. |
| `groups` | Labels that travel in the certificate. Rules name them. |
| `ip` | Overlay address. Written down, never allocated. |
| `ssh` | Server nodes only. `port` defaults to 22. |
| `public_addr` | Underlay address for a lighthouse. |
| `platform` | User nodes only. `os/arch`, for example `linux/amd64`, `darwin/arm64`, or `windows/amd64`. |

A server node needs `ssh` and must not set `platform`. A user node needs `platform` and must not set `ssh`.

An address must sit inside `cidr` and must not be the network or broadcast address.

## rules

```yaml
rules:
  - from: user
    to: web-1
    ports: ["443/tcp"]
  - from: admin
    to: any
    ports: ["22/tcp", "8000-8100/tcp", "any/icmp"]
```

| Key | Meaning |
|---|---|
| `from` | A node name, a group name, or `any`. |
| `to` | A node name, a group name, or `any`. |
| `ports` | A list of `port/proto`, `first-last/proto`, or `any/icmp`. Protocol is `tcp`, `udp`, `icmp`, or `any`. |

The destination enforces the rule. Deny is the default and cannot be written.

A rule from a group that no node carries is an error, because it grants nothing.

## Validation

`meshy validate` reports every problem at once. It checks:

- the schema version against the binary
- names, interface length, and the Nebula version form
- `cidr` as an IPv4 network
- the relationship between certificate lifetimes and the certificate authority lifetime
- duplicate node names and duplicate addresses
- addresses inside the network and away from the network and broadcast addresses
- `kind` against `ssh` and `platform`
- lighthouses as existing server nodes with an underlay address
- every rule endpoint and every port form

The same checks run before `plan` and `apply`. A file that fails validation is never applied.
