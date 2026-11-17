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

// fence opens and closes the plan in a step summary. A raw string cannot hold
// a backtick, so the template joins this constant.
const fence = "```"

// workflow is the workflow file that a mesh repository carries. The meshy
// repository pins the CLI, writes the ssh identity from repository secrets,
// and plans on every pull request from this repository. The plan is the
// report, so a difference passes the job and an exit code of 2 fails it.
const workflow = `name: meshy

on:
  pull_request:
  schedule:
    - cron: "0 3 * * 1"
  workflow_dispatch:

permissions:
  contents: read

env:
  # The CLI comes from this commit of cli-meshy. A mesh repository pins the
  # control plane it runs, so a run cannot pick up a newer CLI by accident.
  MESHY_CLI_REF: 3e55ece0f7293a093b47e097497b78d3e3c32269

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version: stable
      - name: Install meshy
        run: |
          go install github.com/zinzan-vdm/cli-meshy/cmd/meshy@$MESHY_CLI_REF
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"
      - name: Validate
        run: meshy validate

  # A pull request from a fork holds no secrets, so it cannot read a node. The
  # scheduled run and the manual run hold them, and both report drift without
  # a change.
  plan:
    if: github.event_name == 'schedule' || github.event_name == 'workflow_dispatch' || (github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version: stable
      - name: Install meshy
        run: |
          go install github.com/zinzan-vdm/cli-meshy/cmd/meshy@$MESHY_CLI_REF
          echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"
      - name: Write the ssh identity
        env:
          HOSTS: ${{ secrets.MESHY_KNOWN_HOSTS }}
          KEY: ${{ secrets.MESHY_SSH_PRIVATE_KEY }}
          MESHY_KNOWN_HOSTS: ${{ runner.temp }}/meshy/known_hosts
          MESHY_SSH_KEY: ${{ runner.temp }}/meshy/id_ed25519
        run: |
          missing=""
          if [ -z "$HOSTS" ]; then missing="MESHY_KNOWN_HOSTS"; fi
          if [ -z "$KEY" ]; then missing="$missing MESHY_SSH_PRIVATE_KEY"; fi
          if [ -n "$missing" ]; then
            echo "::error::the plan needs these repository secrets: $missing"
            exit 1
          fi
          mkdir -p "$(dirname "$MESHY_KNOWN_HOSTS")"
          umask 077
          printf '%s\n' "$HOSTS" > "$MESHY_KNOWN_HOSTS"
          printf '%s\n' "$KEY" > "$MESHY_SSH_KEY"
      - name: Plan
        env:
          MESHY_KNOWN_HOSTS: ${{ runner.temp }}/meshy/known_hosts
          MESHY_SSH_KEY: ${{ runner.temp }}/meshy/id_ed25519
        run: |
          set +e
          meshy plan --json > "$RUNNER_TEMP/plan.json"
          code=$?
          set -e
          {
            echo "## meshy plan"
            echo
            case "$code" in
              0) echo "The nodes match mesh.yml. A merge changes nothing." ;;
              1) echo "A merge changes the mesh. The plan below is the change." ;;
              *) echo "meshy plan exited $code and produced no plan." ;;
            esac
            echo
            echo '` + fence + `json'
            cat "$RUNNER_TEMP/plan.json"
            echo '` + fence + `'
          } >> "$GITHUB_STEP_SUMMARY"
          if [ "$code" -ge 2 ]; then
            echo "::error::meshy plan produced no plan, exit code $code"
            exit 1
          fi
      - name: Remove the ssh identity
        if: always()
        run: rm -rf "${{ runner.temp }}/meshy"
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
