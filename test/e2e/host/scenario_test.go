package host

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// TestMeshFileDescribesTheRun checks the mesh that the workflow renders against.
// The run cannot prove a join unless the mesh names one lighthouse, one node
// that the host serves, and a rule that lets the probe reach that node.
func TestMeshFileDescribesTheRun(t *testing.T) {
	mesh, err := config.Load(".")
	if err != nil {
		t.Fatalf("load mesh.yml: %v", err)
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		t.Fatalf("mesh.yml has %d problems: %s", len(problems), strings.Join(problems, ", "))
	}

	if len(mesh.Lighthouses) != 1 || mesh.Lighthouses[0].Node != "lighthouse" {
		t.Fatalf("mesh.yml names %v as its lighthouse, want one lighthouse node", mesh.Lighthouses)
	}

	var host, lighthouse config.Node
	for _, node := range mesh.Nodes {
		switch node.Name {
		case "lighthouse":
			lighthouse = node
		case "node-a":
			host = node
		}
	}
	if host.Name == "" {
		t.Fatal("mesh.yml names no node-a, so the host has no node")
	}
	if host.SSH == nil || host.SSH.Host != "host" {
		t.Fatalf("node-a reaches %v, want the compose service host", host.SSH)
	}
	if lighthouse.PublicAddr == "" {
		t.Fatal("the lighthouse has no public_addr, so a node cannot find it")
	}

	allowed := false
	for _, rule := range mesh.Rules {
		if rule.From != "core" || rule.To != "web" {
			continue
		}
		for _, port := range rule.Ports {
			if port == "any/icmp" {
				allowed = true
			}
		}
	}
	if !allowed {
		t.Fatal("no rule lets the lighthouse group send ICMP to the node, so the join probe is dropped")
	}
}

// service is the part of the compose file that this test reads.
type service struct {
	Image       string            `yaml:"image"`
	CapAdd      []string          `yaml:"cap_add"`
	Devices     []string          `yaml:"devices"`
	Volumes     []string          `yaml:"volumes"`
	Environment map[string]string `yaml:"environment"`
}

// TestComposeCarriesThePrivileges checks that the two nodes can run Nebula and
// that no service reads a repository secret. The vault and the shares are made
// in the run, so the file holds no secret.
func TestComposeCarriesThePrivileges(t *testing.T) {
	body, err := os.ReadFile("compose.yml")
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}
	var file struct {
		Services map[string]service `yaml:"services"`
		Secrets  map[string]any     `yaml:"secrets"`
	}
	if err := yaml.Unmarshal(body, &file); err != nil {
		t.Fatalf("parse compose.yml: %v", err)
	}
	if len(file.Secrets) != 0 {
		t.Errorf("compose.yml names %d secrets, want none", len(file.Secrets))
	}

	for _, name := range []string{"lighthouse", "host"} {
		held, ok := file.Services[name]
		if !ok {
			t.Fatalf("compose.yml has no %s service", name)
		}
		if held.Image != "meshy-host-e2e:local" {
			t.Errorf("%s runs image %q, want the tag that CI builds", name, held.Image)
		}
		if !holds(held.CapAdd, "NET_ADMIN") {
			t.Errorf("%s does not add NET_ADMIN", name)
		}
		if !holds(held.Devices, "/dev/net/tun") {
			t.Errorf("%s does not map /dev/net/tun", name)
		}
		if len(held.Environment) != 0 {
			t.Errorf("%s reads %d environment values, want none", name, len(held.Environment))
		}
	}

	runner, ok := file.Services["runner"]
	if !ok {
		t.Fatal("compose.yml has no runner service")
	}
	if runner.Image != "meshy-host-runner:local" {
		t.Errorf("the runner runs image %q, want the tag that CI builds", runner.Image)
	}
	if !holds(runner.Volumes, "./.build/ssh/id_ed25519:/work/ssh/id_ed25519:ro") {
		t.Errorf("the runner mounts %v, want the private key read only", runner.Volumes)
	}
}

// workflow is the part of the workflow file that this test reads.
type workflow struct {
	Jobs map[string]struct {
		Timeout int `yaml:"timeout-minutes"`
		Steps   []struct {
			Name string `yaml:"name"`
			Uses string `yaml:"uses"`
			If   string `yaml:"if"`
			Run  string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

var pinned = regexp.MustCompile(`@[0-9a-f]{40}$`)

// TestWorkflowPinsAndTearsDown checks the job that runs the scenario. One job,
// every action at a commit, a timeout on the run, and a teardown that runs
// whatever happened.
func TestWorkflowPinsAndTearsDown(t *testing.T) {
	body, err := os.ReadFile("../../../.github/workflows/host.yml")
	if err != nil {
		t.Fatalf("read host.yml: %v", err)
	}
	var file workflow
	if err := yaml.Unmarshal(body, &file); err != nil {
		t.Fatalf("parse host.yml: %v", err)
	}
	if len(file.Jobs) != 1 {
		t.Fatalf("host.yml holds %d jobs, want one", len(file.Jobs))
	}

	for name, job := range file.Jobs {
		if job.Timeout <= 0 {
			t.Errorf("job %s has no timeout-minutes", name)
		}
		var timed, teardown bool
		for _, step := range job.Steps {
			if step.Uses != "" && !pinned.MatchString(step.Uses) {
				t.Errorf("job %s uses %q, which is not a 40 character commit", name, step.Uses)
			}
			if strings.HasPrefix(step.Run, "timeout ") {
				timed = true
			}
			if step.If == "always()" && strings.Contains(step.Run, "down -v") {
				teardown = true
			}
		}
		if !timed {
			t.Errorf("job %s has no step that caps the run with timeout", name)
		}
		if !teardown {
			t.Errorf("job %s has no if: always() step that runs down -v", name)
		}
	}
}

// TestRunPrintsSevenAssertions checks the script that the runner runs. The
// output is the record of the run, so every assertion prints its own line.
func TestRunPrintsSevenAssertions(t *testing.T) {
	body, err := os.ReadFile("run.sh")
	if err != nil {
		t.Fatalf("read run.sh: %v", err)
	}
	script := string(body)

	for i := 1; i <= 7; i++ {
		line := "printf 'ok " + string(rune('0'+i)) + " "
		if !strings.Contains(script, line) {
			t.Errorf("run.sh prints no line for assertion %d", i)
		}
	}
	if !strings.Contains(script, "7 assertions passed") {
		t.Error("run.sh does not report the count of assertions that passed")
	}
	for _, need := range []string{"ca init", "unlock --from-env", "apply --node", "install --node", "bootstrap"} {
		if !strings.Contains(script, need) {
			t.Errorf("run.sh never runs %q", need)
		}
	}
}

func holds(list []string, want string) bool {
	for _, entry := range list {
		if entry == want {
			return true
		}
	}
	return false
}
