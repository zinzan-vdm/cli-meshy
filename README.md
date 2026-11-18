# cli-meshy

`meshy` builds and maintains private overlay networks on top of Nebula. One git repository describes one mesh. A pull request plans the change, a merge applies it, and a GitHub Actions runner does the work. `meshy` also runs locally with the same code path.

Status: pre-alpha. `init`, `validate`, `plan`, `ca init`, `ca rotate --phase 1` and `--phase 2`, `issue`, `bootstrap`, `install`, `bundle`, `unlock`, `lock`, and `apply` work.

## What it does

Nebula carries the traffic. Each node holds a certificate that names its address and its groups, and each node enforces its own inbound rules. `meshy` generates the certificates, generates the configuration, writes it to the nodes over SSH, and signals a reload. For a node that it cannot reach, it writes a single archive that a person unpacks.

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

The same workflow runs `plan` on a pull request from the repository and puts the JSON in the run summary. A merge to `main` runs `unlock`, `apply`, and `lock`. A weekly run applies only when the plan reports drift or a renewal. The plan needs two repository secrets, and the apply job also needs three shares. [docs/conf/0002-the-mesh-workflow.md](docs/conf/0002-the-mesh-workflow.md) names them and states what a maintainer must set.

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
3. It writes `/etc/sudoers.d/meshy-<mesh>` with one rule, which allows `reload` and `restart` of `nebula@meshy-<mesh>.service`, and one `install` that places `/usr/local/bin/nebula` from `/etc/nebula/meshy-<mesh>/nebula.incoming`. It checks the rule with `visudo` before it installs the file.
4. It writes `/etc/systemd/system/nebula@meshy-<mesh>.service` and enables it.
5. It runs `systemctl daemon-reload`, then restarts the unit only when it changed and was active.

A second run changes nothing, so the same command repairs a node.

The order for one host is `bootstrap`, then `meshy install`, then `meshy apply`. `bootstrap` refuses a user node, because a user node gets a bundle rather than a unit. The decisions behind the script are in [docs/adr/0009](docs/adr/0009-the-bootstrap-script-owns-the-host.md).

### Install the pinned release

`install` puts the release that `mesh.yml` pins on every server node, and restarts only the nodes that moved:

```sh
meshy install
meshy install --node web-1
meshy install --source ./dist
```

The mesh file is the only place a version is written. `install` reads `nebula.version`, asks each node what it runs with `uname -s` and `uname -m`, and fetches that platform once. It compares the archive with the digest that the project publishes, keeps only `nebula` in `.meshy/bin/<version>/<os>-<arch>/`, and sends that binary to `/usr/local/bin/nebula` on each node.

A node that already reports the pin is left alone. A node that fails is reported, the run continues, and the run exits 1. `--source` reads the archives from a directory and makes no network call. The cache sits inside `.meshy`, so `meshy lock` empties it.

The binary reaches the node as the node user, and one command places it as root. That command names its source, its destination, its owner, and its mode, so the node user cannot place another file. The rules are in [docs/patterns/0012](docs/patterns/0012-the-release-is-verified-before-a-node-sees-it.md) and [0013](docs/patterns/0013-the-node-places-the-binary-through-one-exact-rule.md).

### Make every node match the mesh

`apply` runs over every server node unless `--node` names one. For each node it signs a missing certificate and renews one inside `certificates.renew_before`. It then writes the files that differ and refreshes the unit one time:

```sh
meshy unlock
meshy apply
meshy apply --node web-1
```

The vault must be open. A missing unit stops that node with a message that names `meshy bootstrap`, and the run continues with the next node.

A certificate that the mesh file no longer names is blocked. The run adds its fingerprint to the blocklist of every node, then deletes the certificate and the key from the vault. The fingerprint stays in the vault, so a node that returns is still blocked. These rules are in [docs/patterns/0014](docs/patterns/0014-a-block-outlives-the-certificate.md) and [0015](docs/patterns/0015-the-blocklist-is-a-file-the-config-names.md).

`apply` writes a file only when the content differs. A second run changes nothing and does not reload. Each file goes to a temporary name and a rename, so a failed write leaves the previous file in place.

The exit code is 0 when every node matches or was updated. It is 1 when at least one node failed, and the run still reached the others. It is 2 when the run never started. `--dry-run` prints the plan and writes nothing. `--json` prints the report, one entry for each node.

`apply` needs the pinned `nebula-cert`, because it signs and renews. It finds the tool in `.meshy/bin/<version>/` or on `PATH`.

The write runs as the node user and needs no sudo. Only the unit command uses `sudo`: a reload when the unit runs, and a restart when it does not. The rules are in [docs/patterns/0009](docs/patterns/0009-a-write-to-a-node-replaces-the-file-by-rename.md), [0010](docs/patterns/0010-the-unit-reloads-once-after-the-last-write.md), and [0011](docs/patterns/0011-a-stopped-unit-is-started-not-reloaded.md).

### Rotate the authority

A rotation is three phases, and each one leaves the mesh working. Phase 1 adds the next authority to the trust bundle and changes no certificate:

```sh
meshy unlock
meshy ca rotate --phase 1
```

The command creates the next authority, seals it in the vault, adds it to `ca.trust` in `mesh.yml`, and writes the new bundle to every server node. It reads the authority list back from each node and prints the phase that comes next. Phase 1 signs no certificate, so a node that the run does not reach keeps its old certificate and keeps working.

Phase 2 makes that authority active and signs a new certificate for every server node:

```sh
meshy ca rotate --phase 2
```

The command changes `ca.active` in `mesh.yml`, signs a certificate from the new authority, writes it to the node, and reloads the unit. It reads the signing authority back from every server node. The vault keeps the old certificate until phase 3, so the old authority stays valid.

The command refuses to run when `ca.trust` already holds two authorities, and when the vault is locked. The exit code is 0 when every node took the bundle, 1 when at least one node failed, and 2 when the run never started.

Phase 2 refuses while `ca.trust` holds one authority, and it stops at the first node that does not come back. A mesh that the new authority already signs is reported as no change.

The decisions are in [docs/adr/0011](docs/adr/0011-a-trust-bundle-is-one-file-of-certificates.md) and [docs/adr/0012](docs/adr/0012-phase-2-changes-the-active-authority.md). The rules for the edit and the write are in [docs/patterns/0020](docs/patterns/0020-an-edit-to-mesh-yml-keeps-every-other-byte.md) and [0021](docs/patterns/0021-a-rotation-signs-then-reloads.md).

### Join a user node

A user node is a machine that `meshy` cannot reach over SSH, so the control plane cannot configure it. `bundle` writes one archive for it:

```sh
meshy unlock
meshy bundle laptop-1
meshy bundle laptop-1 --no-binary
```

The command writes `bundles/<mesh>/<node>.tar.gz` and prints that path. The archive holds the rendered `config.yml`, `ca.crt`, the node certificate, the node key, `blocklist.txt`, the pinned binary, and one installer for each platform. The file has mode 0600 and the key has mode 0600 inside the archive, because both hold a private key.

`bundle` renews a certificate that is inside `certificates.renew_before` before it builds the archive, so the archive and the vault hold the same certificate. It reads the binary from `.meshy/bin/<version>/<os>-<arch>/`, which is the cache that `meshy install` fills, and fetches the release through the same path when the cache is empty. `--no-binary` leaves the binary out, and the installer then downloads the pinned release and checks its checksum.

A server node is refused with a message that names `apply`. A locked vault is refused with a message that names `meshy unlock`.

The person unpacks the archive and runs one file: `install.sh` on Linux, `install.command` on macOS, or `install.ps1` on Windows. The installer needs root, places the files in `/etc/nebula/meshy-<mesh>/`, installs the service, and starts it. On Linux it writes `nebula@meshy-<mesh>.service`, which is the unit that `apply` reloads. Delivery of the archive is out of band.

The decisions are in [docs/adr/0010](docs/adr/0010-one-archive-carries-every-installer.md). The rules are in [docs/patterns/0017](docs/patterns/0017-a-bundle-is-built-in-memory-and-written-once.md) and [0018](docs/patterns/0018-an-installer-checks-its-rights-before-it-writes.md).

## Commands

| Command | State |
|---|---|
| `init` | works |
| `validate` | works |
| `ca init` | works |
| `unlock`, `lock` | works |
| `issue` | works |
| `plan` | works |
| `apply` | works |
| `install` | works |
| `bundle` | works |
| `bootstrap` | works |
| `ca rotate --phase 1`, `ca rotate --phase 2` | works |
| `ca rotate --phase 3` | planned |

`--mesh <dir>` selects the directory that holds `mesh.yml`. It defaults to the current directory.

The work is tracked in `.beans/`, one markdown file per ticket with YAML frontmatter. `beans list` shows the tree, and `beans show <id>` shows one ticket.

## Licence

GPL-3.0.

The Shamir secret sharing in `internal/shamir` comes from [HashiCorp Vault](https://github.com/hashicorp/vault/blob/v1.15.0/shamir/shamir.go) at v1.15.0, under MPL-2.0. The file keeps the upstream copyright and licence line.
