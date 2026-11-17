package nebula

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// ConfigRoot is where a node keeps its Nebula files.
const ConfigRoot = "/etc/nebula"

// ConfigDir returns the directory that holds one node's files for a mesh.
func ConfigDir(meshName string) string {
	return ConfigRoot + "/meshy-" + meshName
}

// Render turns a mesh and one node into the Nebula config that the node must
// run. It takes values and returns bytes. It reads no file, opens no
// connection, and reads no clock.
func Render(mesh config.Mesh, node config.Node) ([]byte, error) {
	if len(mesh.Lighthouses) == 0 {
		return nil, fmt.Errorf("mesh %q has no lighthouse", mesh.Name)
	}

	staticHostMap := map[string][]string{}
	hosts := make([]string, 0, len(mesh.Lighthouses))
	amLighthouse := false
	lighthousePort := mesh.Lighthouses[0].Port

	for _, entry := range mesh.Lighthouses {
		peer, found := findNode(mesh, entry.Node)
		if !found {
			return nil, fmt.Errorf("lighthouse %q is not a node", entry.Node)
		}
		if peer.PublicAddr == "" {
			return nil, fmt.Errorf("lighthouse %q has no public_addr", peer.Name)
		}
		if entry.Node == node.Name {
			amLighthouse = true
			lighthousePort = entry.Port
		}
		hosts = append(hosts, peer.IP)
		staticHostMap[peer.IP] = []string{peer.PublicAddr}
	}

	if amLighthouse {
		hosts = nil
		staticHostMap = nil
	}

	inbound, err := Expand(mesh, node)
	if err != nil {
		return nil, err
	}

	dir := ConfigDir(mesh.Name)
	doc := document{
		PKI: pki{
			CA:                dir + "/ca.crt",
			Cert:              dir + "/" + node.Name + ".crt",
			Key:               dir + "/" + node.Name + ".key",
			DisconnectInvalid: true,
		},
		StaticHostMap: staticHostMap,
		Lighthouse: lighthouse{
			AmLighthouse: amLighthouse,
			Hosts:        hosts,
			Port:         lighthousePort,
		},
		Punchy: punchy{Punch: true, Respond: true},
		Tun:    tun{Dev: mesh.Interface},
		Firewall: firewall{
			Inbound: inbound,
			Outbound: []Rule{
				{Port: AnyPort(), Proto: "any", Host: "any"},
			},
		},
		Logging: logging{Level: "info", Format: "text"},
	}

	if amLighthouse {
		doc.Listen = &listen{Host: "0.0.0.0", Port: lighthousePort}
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return nil, fmt.Errorf("render config for %q: %w", node.Name, err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("render config for %q: %w", node.Name, err)
	}
	return buf.Bytes(), nil
}

func findNode(mesh config.Mesh, name string) (config.Node, bool) {
	for _, node := range mesh.Nodes {
		if node.Name == name {
			return node, true
		}
	}
	return config.Node{}, false
}

type document struct {
	PKI           pki                 `yaml:"pki"`
	StaticHostMap map[string][]string `yaml:"static_host_map,omitempty"`
	Lighthouse    lighthouse          `yaml:"lighthouse"`
	Listen        *listen             `yaml:"listen,omitempty"`
	Punchy        punchy              `yaml:"punchy"`
	Tun           tun                 `yaml:"tun"`
	Firewall      firewall            `yaml:"firewall"`
	Logging       logging             `yaml:"logging"`
}

type pki struct {
	CA                string `yaml:"ca"`
	Cert              string `yaml:"cert"`
	Key               string `yaml:"key"`
	DisconnectInvalid bool   `yaml:"disconnect_invalid"`
}

type lighthouse struct {
	AmLighthouse bool     `yaml:"am_lighthouse"`
	Hosts        []string `yaml:"hosts"`
	Port         int      `yaml:"port"`
}

type listen struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type punchy struct {
	Punch   bool `yaml:"punch"`
	Respond bool `yaml:"respond"`
}

type tun struct {
	Dev string `yaml:"dev"`
}

type firewall struct {
	Inbound  []Rule `yaml:"inbound"`
	Outbound []Rule `yaml:"outbound"`
}

type logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func ConfigPath(meshName, nodeName string) string {
	return strings.TrimSuffix(ConfigDir(meshName), "/") + "/config.yml"
}
