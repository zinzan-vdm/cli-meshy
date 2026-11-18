package e2e

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// service is the part of the compose file that this test reads.
type service struct {
	Image       string   `yaml:"image"`
	CapAdd      []string `yaml:"cap_add"`
	Devices     []string `yaml:"devices"`
	NetworkMode string   `yaml:"network_mode"`
	Command     []string `yaml:"command"`
}

// TestComposeCarriesThePrivileges checks the two settings that a Nebula
// container cannot run without. A missing one fails the job with a message
// about a device, so this test names it first.
func TestComposeCarriesThePrivileges(t *testing.T) {
	body, err := os.ReadFile("compose.yml")
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}
	var file struct {
		Services map[string]service `yaml:"services"`
	}
	if err := yaml.Unmarshal(body, &file); err != nil {
		t.Fatalf("parse compose.yml: %v", err)
	}

	for _, name := range []string{"lighthouse", "node-a", "node-b"} {
		held, ok := file.Services[name]
		if !ok {
			t.Fatalf("compose.yml has no %s service", name)
		}
		if held.Image != "meshy-e2e:local" {
			t.Errorf("%s runs image %q, want the one tag that CI builds", name, held.Image)
		}
		if !holds(held.CapAdd, "NET_ADMIN") {
			t.Errorf("%s does not add NET_ADMIN", name)
		}
		if !holds(held.Devices, "/dev/net/tun") {
			t.Errorf("%s does not map /dev/net/tun", name)
		}
	}

	harness, ok := file.Services["harness"]
	if !ok {
		t.Fatal("compose.yml has no harness service")
	}
	if harness.NetworkMode != "service:node-b" {
		t.Errorf("the harness runs in %q, so its probes do not leave from node-b", harness.NetworkMode)
	}
	if len(harness.Command) == 0 || harness.Command[0] != "harness" {
		t.Errorf("the harness service runs %v", harness.Command)
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
