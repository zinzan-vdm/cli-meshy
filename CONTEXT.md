# cli-meshy

cli-meshy builds and maintains private overlay networks. One git repository describes one mesh. A command-line tool and a GitHub Actions workflow apply that description to the nodes.

## Language

**Mesh**:
A private overlay network defined by one git repository.
_Avoid_: overlay, network, cluster, VPN

**Node**:
A host that holds a certificate in a mesh.
_Avoid_: host, peer, machine, device, endpoint

**Server node**:
A node that cli-meshy manages over SSH.
_Avoid_: machine node, managed node

**User node**:
A node whose owner installs a bundle by hand. cli-meshy cannot reach it.
_Avoid_: client, user device, unmanaged node

**Lighthouse**:
The node that answers peer discovery for a mesh. It must have a public address.
_Avoid_: coordinator, discovery server, hub

**Group**:
A label carried in a node certificate. Rules use groups to name sources and destinations.
_Avoid_: tag, role, label, class

**Rule**:
A permission for one source to reach one destination on given ports.
_Avoid_: ACL, firewall rule, policy, grant

**Bundle**:
The archive a user node needs. It holds a certificate, a key, a config, the Nebula binary, and an installer.
_Avoid_: config, package, payload, profile

**Vault**:
The encrypted file in the repository that holds the certificate authority key and the node keys.
_Avoid_: secrets file, keystore, store

**Share**:
One part of the key that unseals the vault. A threshold of shares is needed.
_Avoid_: fragment, shard, piece, key part

**Plan**:
A read-only comparison of the repository against the nodes.
_Avoid_: diff, check, dry run, validate

**Apply**:
The write that makes the nodes match the repository.
_Avoid_: deploy, push, sync, reconcile

**Issue**:
The command that mints a certificate. It needs the certificate authority key.
_Avoid_: sign, enroll, provision, register

**Install**:
The command that moves Nebula binaries to the nodes.
_Avoid_: deploy, update, upgrade

**Interface**:
The TUN device name on a node. It is at most fifteen characters.
_Avoid_: tun, device, adapter
