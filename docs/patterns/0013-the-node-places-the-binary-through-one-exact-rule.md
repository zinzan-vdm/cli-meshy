---
tags:
  - install
  - systemd
---

# The node places the binary through one exact rule

In `internal/install`, send the binary to `<dir>/nebula.incoming` with the write path of MESHY-014. Then run one command that names its source, its destination, its owner, and its mode.

## Rationale

`/usr/local/bin` belongs to root, and the node user cannot write it. The rule of MESHY-027 allowed two unit verbs, so a third command is needed.

The rule holds the whole argument list:

```
meshy ALL=(root) NOPASSWD: /usr/bin/systemctl reload nebula@meshy-homelab.service, /usr/bin/systemctl restart nebula@meshy-homelab.service, /usr/bin/install -o root -g root -m 0755 /etc/nebula/meshy-homelab/nebula.incoming /usr/local/bin/nebula
```

sudo matches a command by its path and by its arguments. The node user can therefore place that one file at that one path as root, and it can place no other file anywhere else. A rule with a wildcard, or a rule for a shell, would hand over the host.

The binary travels on the session's stdin as the node user, which needs no privilege. The one command that needs root does one thing.

`install` reads the source and replaces the destination, so a Nebula that is running does not hold the old file busy.

## Enforcement

`TestPlaceCommandNamesTheSourceTheOwnerAndTheMode` asserts the whole command. `TestScriptPlacesTheBinaryThroughOneExactCommand` asserts that the rule holds the owner, the mode, the source and the destination, and that it holds no shell. `TestScriptAllowsReloadAndRestartOfThatUnitOnly` asserts that the rule names exactly three commands.

## See also

- [0012](0012-the-release-is-verified-before-a-node-sees-it.md): the archive that this command places.
- [0009](0009-a-write-to-a-node-replaces-the-file-by-rename.md): the write that carries the bytes.
