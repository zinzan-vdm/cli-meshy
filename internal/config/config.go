package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the file that describes a mesh.
const FileName = "mesh.yml"

const Schema = 1

const (
	KindServer = "server"
	KindUser   = "user"
)

// Mesh is the desired state of one overlay network.
type Mesh struct {
	Schema       int          `yaml:"schema"`
	Name         string       `yaml:"name"`
	CIDR         string       `yaml:"cidr"`
	Interface    string       `yaml:"interface"`
	Nebula       Nebula       `yaml:"nebula"`
	CA           CA           `yaml:"ca"`
	Lighthouses  []Lighthouse `yaml:"lighthouses"`
	Certificates Certificates `yaml:"certificates"`
	Nodes        []Node       `yaml:"nodes"`
	Rules        []Rule       `yaml:"rules"`
}

// Nebula pins the data plane version that every server node runs.
type Nebula struct {
	Version string `yaml:"version"`
}

type CA struct {
	Duration Duration `yaml:"duration"`
	RotateAt string   `yaml:"rotate_at"`
	Trust    []string `yaml:"trust"`
	Active   string   `yaml:"active"`
}

// Lighthouse is a node that answers peer discovery.
type Lighthouse struct {
	Node string `yaml:"node"`
	Port int    `yaml:"port"`
}

type Certificates struct {
	Server      Duration `yaml:"server"`
	User        Duration `yaml:"user"`
	RenewBefore Duration `yaml:"renew_before"`
}

// Node is a host that holds a certificate in the mesh.
type Node struct {
	Name       string   `yaml:"name"`
	Kind       string   `yaml:"kind"`
	Groups     []string `yaml:"groups"`
	IP         string   `yaml:"ip"`
	Platform   string   `yaml:"platform,omitempty"`
	SSH        *SSH     `yaml:"ssh,omitempty"`
	PublicAddr string   `yaml:"public_addr,omitempty"`
}

// SSH is how cli-meshy reaches a server node.
type SSH struct {
	User string `yaml:"user"`
	Host string `yaml:"host"`
	Port int    `yaml:"port,omitempty"`
}

// Rule is a permission for one source to reach one destination on given ports.
type Rule struct {
	From  string   `yaml:"from"`
	To    string   `yaml:"to"`
	Ports []string `yaml:"ports"`
}

// Defaults fills every value that was left out.
func (m *Mesh) Defaults() {
	if m.Schema == 0 {
		m.Schema = Schema
	}
	if m.Nebula.Version == "" {
		m.Nebula.Version = "v1.12.0"
	}
	if m.CA.Duration.Duration == 0 {
		m.CA.Duration = Duration{Duration: mustDuration("20y"), raw: "20y"}
	}
	if m.CA.RotateAt == "" {
		m.CA.RotateAt = "80%"
	}
	if len(m.CA.Trust) == 0 && m.Name != "" {
		m.CA.Trust = []string{"ca-1"}
	}
	if m.CA.Active == "" {
		m.CA.Active = "ca-1"
	}
	if m.Certificates.Server.Duration == 0 {
		m.Certificates.Server = Duration{Duration: mustDuration("1y"), raw: "1y"}
	}
	if m.Certificates.User.Duration == 0 {
		m.Certificates.User = Duration{Duration: mustDuration("1y"), raw: "1y"}
	}
	if m.Certificates.RenewBefore.Duration == 0 {
		m.Certificates.RenewBefore = Duration{Duration: mustDuration("60d"), raw: "60d"}
	}
	if m.Interface == "" && m.Name != "" {
		m.Interface = GenerateInterface(m.Name)
	}
	for i := range m.Nodes {
		if m.Nodes[i].SSH != nil && m.Nodes[i].SSH.Port == 0 {
			m.Nodes[i].SSH.Port = 22
		}
	}
}

// Load reads mesh.yml and applies the defaults.
func Load(dir string) (*Mesh, error) {
	path := filepath.Join(dir, FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var mesh Mesh
	decoder := yaml.NewDecoder(strings.NewReader(string(body)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&mesh); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	mesh.Defaults()
	return &mesh, nil
}

var (
	namePattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	interfacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,14}$`)
	versionPattern   = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	platformPattern  = regexp.MustCompile(`^(linux|darwin|windows|android|ios)/(amd64|arm64|arm|386)$`)
	portPattern      = regexp.MustCompile(`^(\d+|\d+-\d+|any)/(tcp|udp|icmp|any)$`)
	rotatePattern    = regexp.MustCompile(`^\d{1,3}%$`)
)

// Validate reports every problem in the mesh file. An empty result means the
// file is correct.
func (m *Mesh) Validate() []string {
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if m.Schema != Schema {
		add("schema is %d, this build understands %d", m.Schema, Schema)
	}
	if !namePattern.MatchString(m.Name) {
		add("name %q must start with a letter and hold only a-z, 0-9, and dash, at most 32 characters", m.Name)
	}
	if !interfacePattern.MatchString(m.Interface) || len(m.Interface) > 15 {
		add("interface %q must start with a letter, hold only a-z, 0-9, and dash, and be at most 15 characters", m.Interface)
	}
	if !versionPattern.MatchString(m.Nebula.Version) {
		add("nebula.version %q must look like v1.12.0", m.Nebula.Version)
	}

	cidr, cidrErr := parseCIDR(m.CIDR)
	if cidrErr != nil {
		add("cidr %q is not an IPv4 network: %v", m.CIDR, cidrErr)
	}

	if m.CA.Duration.Duration <= 0 {
		add("ca.duration must be more than zero")
	}
	if !rotatePattern.MatchString(m.CA.RotateAt) {
		add("ca.rotate_at %q must look like 80%%", m.CA.RotateAt)
	}
	if len(m.CA.Trust) == 0 {
		add("ca.trust must name at least one certificate authority")
	}
	trusted := map[string]bool{}
	for _, name := range m.CA.Trust {
		if name == "" {
			add("ca.trust holds an empty name")
			continue
		}
		if trusted[name] {
			add("ca.trust names %q twice", name)
		}
		trusted[name] = true
	}
	if m.CA.Active != "" && !trusted[m.CA.Active] {
		add("ca.active %q is not in ca.trust", m.CA.Active)
	}

	if m.Certificates.Server.Duration <= 0 {
		add("certificates.server must be more than zero")
	}
	if m.Certificates.User.Duration <= 0 {
		add("certificates.user must be more than zero")
	}
	if m.Certificates.RenewBefore.Duration <= 0 {
		add("certificates.renew_before must be more than zero")
	}
	shortest := m.Certificates.Server.Duration
	if m.Certificates.User.Duration < shortest {
		shortest = m.Certificates.User.Duration
	}
	if m.Certificates.RenewBefore.Duration >= shortest {
		add("certificates.renew_before %s must be shorter than the shortest certificate lifetime %s",
			m.Certificates.RenewBefore, FormatDuration(shortest))
	}
	longest := m.Certificates.Server.Duration
	if m.Certificates.User.Duration > longest {
		longest = m.Certificates.User.Duration
	}
	if m.CA.Duration.Duration <= longest {
		add("ca.duration %s must outlast the longest certificate lifetime %s",
			m.CA.Duration, FormatDuration(longest))
	}

	nodes := map[string]*Node{}
	addresses := map[string]string{}
	groups := map[string]bool{}
	for i := range m.Nodes {
		node := &m.Nodes[i]
		if !namePattern.MatchString(node.Name) {
			add("node %q must start with a letter and hold only a-z, 0-9, and dash, at most 32 characters", node.Name)
		}
		if _, seen := nodes[node.Name]; seen {
			add("node %q is named twice", node.Name)
		}
		nodes[node.Name] = node
		for _, group := range node.Groups {
			groups[group] = true
		}

		switch node.Kind {
		case KindServer:
			if node.SSH == nil {
				add("node %q is a server node and needs an ssh block", node.Name)
			} else {
				if node.SSH.User == "" {
					add("node %q needs ssh.user", node.Name)
				}
				if node.SSH.Host == "" {
					add("node %q needs ssh.host", node.Name)
				}
				if node.SSH.Port < 1 || node.SSH.Port > 65535 {
					add("node %q has ssh.port %d, which is out of range", node.Name, node.SSH.Port)
				}
			}
			if node.Platform != "" {
				add("node %q is a server node and must not set platform", node.Name)
			}
		case KindUser:
			if node.SSH != nil {
				add("node %q is a user node and must not set ssh", node.Name)
			}
			if !platformPattern.MatchString(node.Platform) {
				add("node %q needs platform such as darwin/arm64, got %q", node.Name, node.Platform)
			}
		default:
			add("node %q has kind %q, use server or user", node.Name, node.Kind)
		}

		if cidrErr == nil {
			ip, ipErr := parseIP(node.IP)
			if ipErr != nil {
				add("node %q has ip %q: %v", node.Name, node.IP, ipErr)
			} else if !cidr.Contains(ip) {
				add("node %q has ip %s, which is outside cidr %s", node.Name, node.IP, m.CIDR)
			} else if ip == cidr.Addr() || ip == prefixLast(cidr) {
				add("node %q has ip %s, which is a network or broadcast address", node.Name, node.IP)
			}
		}
		if other, seen := addresses[node.IP]; seen {
			add("nodes %q and %q share ip %s", other, node.Name, node.IP)
		}
		addresses[node.IP] = node.Name
	}

	if len(m.Lighthouses) == 0 {
		add("lighthouses must name at least one node")
	}
	for _, lighthouse := range m.Lighthouses {
		node, found := nodes[lighthouse.Node]
		if !found {
			add("lighthouse %q is not a node", lighthouse.Node)
			continue
		}
		if node.Kind != KindServer {
			add("lighthouse %q must be a server node", lighthouse.Node)
		}
		if node.PublicAddr == "" {
			add("lighthouse %q needs public_addr", lighthouse.Node)
		}
		if lighthouse.Port < 1 || lighthouse.Port > 65535 {
			add("lighthouse %q has port %d, which is out of range", lighthouse.Node, lighthouse.Port)
		}
	}

	for i, rule := range m.Rules {
		where := fmt.Sprintf("rule %d (%s -> %s)", i+1, rule.From, rule.To)
		if !m.knownEndpoint(rule.From, nodes, groups) {
			add("%s: from %q is not a node or a group", where, rule.From)
		}
		if !m.knownEndpoint(rule.To, nodes, groups) {
			add("%s: to %q is not a node or a group", where, rule.To)
		}
		if len(rule.Ports) == 0 {
			add("%s has no ports", where)
		}
		for _, port := range rule.Ports {
			if !portPattern.MatchString(port) {
				add("%s has port %q, use a form such as 443/tcp, 8000-8100/tcp, or any/icmp", where, port)
			}
		}
	}

	sort.Strings(problems)
	return problems
}

func (m *Mesh) knownEndpoint(name string, nodes map[string]*Node, groups map[string]bool) bool {
	if name == "any" {
		return true
	}
	if _, found := nodes[name]; found {
		return true
	}
	return groups[name]
}

// GenerateInterface makes a stable name that fits the fifteen character limit
// of Linux interfaces.
func GenerateInterface(meshName string) string {
	const prefix = "meshy-"
	if len(meshName) <= 8 {
		return prefix + meshName
	}
	trimmed := strings.TrimRight(meshName[:6], "-")
	return prefix + trimmed + "-" + shortHash(meshName)
}

func shortHash(s string) string {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return fmt.Sprintf("%02x", h%256)
}
