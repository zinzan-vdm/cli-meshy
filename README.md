# cli-meshy

`meshy` builds and maintains private overlay networks on top of Nebula. One git repository describes one mesh. A pull request plans the change, a merge applies it, and a GitHub Actions runner does the work. `meshy` also runs locally with the same code path.

Status: pre-alpha. `init`, `validate`, `plan`, `ca init`, `issue`, `bootstrap`, `unlock`, `lock`, and `apply --node` work.

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

`validate` returns a non-zero exit code and one line per problem. The workflow that `init` writes runs it on every pull request.

The same workflow runs `plan` on a pull request from the repository and puts the JSON in the run summary. The plan needs two repository secrets. [docs/conf/0002-the-mesh-workflow.md](docs/conf/0002-the-mesh-workflow.md) names them and states what a maintainer must set.

The field reference is in [docs/conf/0001-mesh-yml.md](docs/conf/0001-mesh-yml.md).

### Prepare a server node

A fresh host holds no node user, no sudoers rule, no config directory, and no unit. `bootstrap` prints a script that creates all four:

```sh
meshy bootstrap web-1 | sudo sh
```

The command prints the script and never runs it. Read the script before you pipe it:

```sh
meshy bootstrap web-1
```

The script does five things on the host:

1. It creates the user from `ssh.user`, unless the user exists.
2. It creates `/etc/nebula/meshy-<mesh>/` with mode 0700 and gives it to the node user.
3. It writes `/etc/sudoers.d/meshy-<mesh>` with one rule, which allows `reload` and `restart` of `nebula@meshy-<mesh>.service` and no other command. It checks the rule with `visudo` before it installs the file.
4. It writes `/etc/systemd/system/nebula@meshy-<mesh>.service` and enables it.
5. It runs `systemctl daemon-reload`, then restarts the unit only when it changed and was active.

A second run changes nothing, so the same command repairs a node.

The order for one host is `bootstrap`, then `meshy install`, then `meshy apply`. `bootstrap` refuses a user node, because a user node gets a bundle rather than a unit. The decisions behind the script are in [docs/adr/0009](docs/adr/0009-the-bootstrap-script-owns-the-host.md).

### Apply the config to one node

`apply` writes the config, the certificate, the authority certificate, and the key to one node, then reloads the unit one time:

```sh
meshy unlock
meshy issue web-1
meshy apply --node web-1
```

The vault must be open, and it must hold a certificate for the node. A missing certificate stops the run with a message that names `meshy issue`. A missing unit stops the run with a message that names `meshy bootstrap`.

`apply` writes a file only when the content differs. A second run changes nothing and does not reload. Each file goes to a temporary name and a rename, so a failed write leaves the previous file in place.

The exit code is 0 when the node matches or was updated. It is 1 when the node failed or the unit is missing. It is 2 when the run never started. `--node` is required, and this build writes one node per run.

The write runs as the node user and needs no sudo. Only the reload uses `sudo -n systemctl reload`. The two rules are in [docs/patterns/0009](docs/patterns/0009-a-write-to-a-node-replaces-the-file-by-rename.md) and [0010](docs/patterns/0010-the-unit-reloads-once-after-the-last-write.md).

## Commands

| Command | State |
|---|---|
| `init` | works |
| `validate` | works |
| `ca init` | works |
| `unlock`, `lock` | works |
| `issue` | works |
| `plan` | works |
| `apply` | works for one node |
| `install` | planned |
| `bundle` | planned |
| `bootstrap` | works |
| `ca rotate` | planned |

`--mesh <dir>` selects the directory that holds `mesh.yml`. It defaults to the current directory.

The work is tracked in `.beans/`, one markdown file per ticket with YAML frontmatter. `beans list` shows the tree, and `beans show <id>` shows one ticket.

## Licence

GPL-3.0.

The Shamir secret sharing in `internal/shamir` comes from [HashiCorp Vault](https://github.com/hashicorp/vault/blob/v1.15.0/shamir/shamir.go) at v1.15.0, under MPL-2.0. The file keeps the upstream copyright and licence line.
