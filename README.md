# cli-meshy

`meshy` builds and maintains private overlay networks on top of Nebula. One git repository describes one mesh. A pull request plans the change, a merge applies it, and a GitHub Actions runner does the work. `meshy` also runs locally with the same code path.

Status: pre-alpha. `init` and `validate` work. Nothing touches a node yet.

## What it does

Nebula carries the traffic. Each node holds a certificate that names its address and its groups, and each node enforces its own inbound rules. `meshy` generates the certificates, generates the configuration, writes it to the nodes over SSH, and signals a reload. It also produces a single archive for people who install by hand.

Access is written as a rule from a source to a destination on a port:

```yaml
rules:
  - from: user
    to: web-1
    ports: ["443/tcp"]
```

The destination enforces the rule, so a source cannot skip the check.

## How it works

- The repository is the mesh. `mesh.yml` is the desired state. `secrets.age` is the vault.
- Nebula is the data plane. We do not build a tunnel, a discovery service, or a packet filter.
- GitHub Actions is the primary executor. A merge applies. A workflow can also run on a schedule.
- The pull request is the approval gate. A review of `mesh.yml` is the moment an identity is approved.
- `plan` reads the nodes and prints the difference. `apply` writes. Neither keeps a state file.
- The vault holds the certificate authority key and the node keys. Five shares seal it, and any three open it.

## Build

Go 1.27.4 or later.

```sh
go build ./...
go test ./...
go run ./cmd/meshy version
```

## Use

Create a mesh repository in an empty directory:

```sh
go run ./cmd/meshy init --name homelab --cidr 10.42.0.0/24 --owner your-handle ./homelab
```

That writes `mesh.yml`, `.gitignore`, `.github/CODEOWNERS`, and `.github/workflows/meshy.yml`. Edit the node block, then check the file:

```sh
go run ./cmd/meshy validate --mesh ./homelab
```

`validate` returns a non-zero exit code and one line per problem. Run it in every pull request.

The field reference is in [docs/conf/0001-mesh-yml.md](docs/conf/0001-mesh-yml.md).

## Commands

| Command | State |
|---|---|
| `init` | works |
| `validate` | works |
| `ca init` | planned |
| `unlock`, `lock` | planned |
| `issue` | planned |
| `plan` | next |
| `apply` | planned |
| `install` | planned |
| `bundle` | planned |
| `bootstrap` | planned |
| `ca rotate` | planned |

`--mesh <dir>` selects the directory that holds `mesh.yml`. It defaults to the current directory.

The work is tracked in `.beans/`, one markdown file per ticket with YAML frontmatter. `beans list` shows the tree, and `beans show <id>` shows one ticket.

## Licence

GPL-3.0.

The Shamir secret sharing in `internal/shamir` comes from [HashiCorp Vault](https://github.com/hashicorp/vault/blob/v1.15.0/shamir/shamir.go) at v1.15.0, under MPL-2.0. The file keeps the upstream copyright and licence line.
