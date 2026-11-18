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
bundles/
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
// report, so a difference passes the job and an exit code of 2 fails it. A
// merge to main, a weekly run, and a manual run reach the apply job, which
// unseals the vault from shares and locks it again.
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

  # A merge is the approval, so only a trusted event reaches this job. A
  # pull request never does, and never gets a share.
  apply:
    if: github.event_name == 'push' || github.event_name == 'schedule' || github.event_name == 'workflow_dispatch'
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
            echo "::error::the apply needs these repository secrets: $missing"
            exit 1
          fi
          mkdir -p "$(dirname "$MESHY_KNOWN_HOSTS")"
          umask 077
          printf '%s\n' "$HOSTS" > "$MESHY_KNOWN_HOSTS"
          printf '%s\n' "$KEY" > "$MESHY_SSH_KEY"
      # The plan runs with a closed vault, so a quiet week never unseals. A
      # merge applies whatever the plan says, because the review approved it.
      - name: Plan
        id: plan
        env:
          MESHY_KNOWN_HOSTS: ${{ runner.temp }}/meshy/known_hosts
          MESHY_SSH_KEY: ${{ runner.temp }}/meshy/id_ed25519
          MESHY_EVENT: ${{ github.event_name }}
        run: |
          set +e
          meshy plan --json > "$RUNNER_TEMP/plan.json"
          code=$?
          set -e
          if [ "$code" -ge 2 ]; then
            echo "::error::meshy plan produced no plan, exit code $code"
            exit 1
          fi
          # The plan exit code carries the answer: 0 is a match, 1 is a
          # difference, and anything above 1 stopped the step. So the gate
          # needs no JSON reader and no tool beyond the shell.
          apply=false
          if [ "$MESHY_EVENT" = "push" ] || [ "$code" -eq 1 ]; then
            apply=true
          fi
          {
            echo "## meshy plan"
            echo
            case "$code" in
              0) echo "The nodes match mesh.yml." ;;
              1) echo "The plan holds at least one difference: drift or a renewal." ;;
            esac
            if [ "$apply" = "true" ]; then
              echo
              echo "This run applies the plan."
            else
              echo
              echo "No drift and no renewal, so this run applies nothing."
            fi
            echo
            echo '` + fence + `json'
            cat "$RUNNER_TEMP/plan.json"
            echo '` + fence + `'
          } >> "$GITHUB_STEP_SUMMARY"
          echo "apply=$apply" >> "$GITHUB_OUTPUT"
      - name: Unlock
        if: steps.plan.outputs.apply == 'true'
        env:
          MESHY_SHARE_1: ${{ secrets.MESHY_SHARE_1 }}
          MESHY_SHARE_2: ${{ secrets.MESHY_SHARE_2 }}
          MESHY_SHARE_3: ${{ secrets.MESHY_SHARE_3 }}
        run: |
          missing=""
          if [ -z "$MESHY_SHARE_1" ]; then missing="MESHY_SHARE_1"; fi
          if [ -z "$MESHY_SHARE_2" ]; then missing="$missing MESHY_SHARE_2"; fi
          if [ -z "$MESHY_SHARE_3" ]; then missing="$missing MESHY_SHARE_3"; fi
          if [ -n "$missing" ]; then
            echo "::error::the apply needs these repository secrets: $missing"
            exit 1
          fi
          meshy unlock --from-env
      # apply signs and renews, so it needs the tool that the mesh pins. The
      # archive and the member are checked against the published digests, and
      # the tool goes where ca.Find looks. The step runs after Unlock, because
      # Unlock refuses a .meshy that already holds a file.
      - name: Install nebula-cert
        if: steps.plan.outputs.apply == 'true'
        run: |
          version="$(awk '/^[^[:space:]#]/{b=$1} b=="nebula:" && /^[[:space:]]+version:/{print $2; exit}' mesh.yml | tr -d '"')"
          if [ -z "$version" ]; then
            echo "::error::mesh.yml holds no nebula.version"
            exit 1
          fi
          asset="nebula-linux-amd64.tar.gz"
          base="https://github.com/slackhq/nebula/releases/download/$version"
          curl -fsSL "$base/SHASUM256.txt" -o "$RUNNER_TEMP/SHASUM256.txt"
          curl -fsSL "$base/$asset" -o "$RUNNER_TEMP/$asset"
          want="$(awk -v name="$asset" '$2 == name {print $1}' "$RUNNER_TEMP/SHASUM256.txt")"
          if [ -z "$want" ]; then
            echo "::error::SHASUM256.txt names no $asset"
            exit 1
          fi
          echo "$want  $RUNNER_TEMP/$asset" | sha256sum -c -
          tar -xzf "$RUNNER_TEMP/$asset" -C "$RUNNER_TEMP" nebula-cert
          want="$(awk -v name="$asset/nebula-cert" '$2 == name {print $1}' "$RUNNER_TEMP/SHASUM256.txt")"
          if [ -z "$want" ]; then
            echo "::error::SHASUM256.txt names no $asset/nebula-cert"
            exit 1
          fi
          echo "$want  $RUNNER_TEMP/nebula-cert" | sha256sum -c -
          install -D -m 0755 "$RUNNER_TEMP/nebula-cert" ".meshy/bin/$version/linux-amd64/nebula-cert"
      - name: Apply
        if: steps.plan.outputs.apply == 'true'
        env:
          MESHY_KNOWN_HOSTS: ${{ runner.temp }}/meshy/known_hosts
          MESHY_SSH_KEY: ${{ runner.temp }}/meshy/id_ed25519
        run: |
          set +e
          meshy apply --json > "$RUNNER_TEMP/apply.json"
          code=$?
          set -e
          {
            echo "## meshy apply"
            echo
            echo '` + fence + `json'
            cat "$RUNNER_TEMP/apply.json"
            echo '` + fence + `'
          } >> "$GITHUB_STEP_SUMMARY"
          if [ "$code" -ne 0 ]; then
            echo "::error::meshy apply exited $code, see the report above"
            exit "$code"
          fi
      # The vault closes even when the apply step failed, so a failed run does
      # not leave the plaintext on the runner.
      - name: Lock
        if: always()
        run: meshy lock
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
