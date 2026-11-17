package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// meshFile is the template that init writes.
const meshFile = `schema: 1

name: {{NAME}}
cidr: {{CIDR}}
interface: {{INTERFACE}}

nebula:
  version: {{NEBULA}}

ca:
  duration: 20y
  rotate_at: 80%
  trust: [ca-1]
  active: ca-1

lighthouses:
  - node: {{LIGHTHOUSE}}
    port: 4242

certificates:
  server: 1y
  user: 1y
  renew_before: 60d

nodes:
  - name: {{LIGHTHOUSE}}
    kind: server
    groups: [core]
    ip: {{FIRST_IP}}
    ssh:
      user: meshy
      host: 203.0.113.10
    public_addr: 203.0.113.10:4242

rules: []
`

const gitignore = `.meshy/
*.key
*.crt
*.local.yml
`

const codeowners = `# Owner review is the gate that mints identity. See docs/adr/0004.
/mesh.yml        @{{OWNER}}
/secrets.age     @{{OWNER}}
`

const workflow = `name: meshy

on:
  pull_request:
  push:
    branches: [main]
  schedule:
    - cron: "0 3 * * 1"
  workflow_dispatch:

permissions:
  contents: read

jobs:
  plan:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - name: Install meshy
        run: echo "TODO install the pinned meshy release"
      - name: Plan
        run: meshy plan --json

  apply:
    if: github.event_name != 'pull_request' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - name: Install meshy
        run: echo "TODO install the pinned meshy release"
      - name: Unseal the vault
        env:
          MESHY_SHARE_1: ${{ secrets.MESHY_SHARE_1 }}
          MESHY_SHARE_2: ${{ secrets.MESHY_SHARE_2 }}
          MESHY_SHARE_3: ${{ secrets.MESHY_SHARE_3 }}
        run: meshy unlock --from-env
      - name: Apply
        run: meshy apply
      - name: Lock
        if: always()
        run: meshy lock
`

// Options holds the values that init writes into a new mesh file.
type Options struct {
	Dir        string
	Name       string
	CIDR       string
	Lighthouse string
	Owner      string
}

// Init writes a mesh repository into the target directory. It does not
// overwrite files that already exist.
func Init(opts Options) ([]string, error) {
	if err := opts.setDefaults(); err != nil {
		return nil, err
	}

	mesh := config.Mesh{Name: opts.Name, CIDR: opts.CIDR}
	mesh.Defaults()

	first, err := firstUsable(opts.CIDR)
	if err != nil {
		return nil, err
	}

	replacements := map[string]string{
		"{{NAME}}":       opts.Name,
		"{{CIDR}}":       opts.CIDR,
		"{{INTERFACE}}":  mesh.Interface,
		"{{NEBULA}}":     mesh.Nebula.Version,
		"{{LIGHTHOUSE}}": opts.Lighthouse,
		"{{FIRST_IP}}":   first,
		"{{OWNER}}":      opts.Owner,
	}

	files := []struct {
		path string
		body string
	}{
		{config.FileName, meshFile},
		{".gitignore", gitignore},
		{filepath.Join(".github", "CODEOWNERS"), codeowners},
		{filepath.Join(".github", "workflows", "meshy.yml"), workflow},
	}

	var written []string
	for _, file := range files {
		path := filepath.Join(opts.Dir, file.path)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return written, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		body := file.body
		for from, to := range replacements {
			body = strings.ReplaceAll(body, from, to)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, file.path)
	}

	return written, nil
}

func (o *Options) setDefaults() error {
	if o.Dir == "" {
		o.Dir = "."
	}
	if o.Name == "" {
		base, err := filepath.Abs(o.Dir)
		if err != nil {
			return err
		}
		o.Name = filepath.Base(base)
	}
	if o.CIDR == "" {
		o.CIDR = "10.42.0.0/24"
	}
	if o.Lighthouse == "" {
		o.Lighthouse = "core-1"
	}
	if o.Owner == "" {
		o.Owner = "owner"
	}
	return nil
}

func firstUsable(cidr string) (string, error) {
	prefix, err := parsePrefix(cidr)
	if err != nil {
		return "", err
	}
	return prefix.Addr().Next().String(), nil
}
