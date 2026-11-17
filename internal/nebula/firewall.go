package nebula

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
)

// Port is one entry in a Nebula firewall rule. It is a number, a range, or
// "any". It marshals to a number when it holds one, so the config reads the way
// the Nebula examples do.
type Port struct {
	Number int
	Range  string
	Any    bool
}

// NumberPort returns a single port.
func NumberPort(n int) Port { return Port{Number: n} }

// RangePort returns a range such as 8000-8100.
func RangePort(r string) Port { return Port{Range: r} }

// AnyPort returns the port that matches everything.
func AnyPort() Port { return Port{Any: true} }

// IsZero reports that the rule carries no port at all, which is the case for
// ICMP. Nebula warns when an ICMP rule carries a port, so the field is left out.
func (p Port) IsZero() bool { return p.Number == 0 && p.Range == "" && !p.Any }

// String returns the port in the form a person writes.
func (p Port) String() string {
	switch {
	case p.Any:
		return "any"
	case p.Range != "":
		return p.Range
	case p.Number == 0:
		return ""
	default:
		return strconv.Itoa(p.Number)
	}
}

// MarshalYAML writes a number as a number and everything else as a string.
func (p Port) MarshalYAML() (any, error) {
	switch {
	case p.Any:
		return "any", nil
	case p.Range != "":
		return p.Range, nil
	default:
		return p.Number, nil
	}
}

// Rule is one firewall entry in the Nebula form.
type Rule struct {
	Port  Port   `yaml:"port,omitempty"`
	Proto string `yaml:"proto"`
	Host  string `yaml:"host,omitempty"`
	Group string `yaml:"group,omitempty"`
}

// Key returns a stable string for the rule, used to collapse duplicates and to
// sort the result. The source name comes first, so a group and a host sort by
// the name a person wrote.
func (r Rule) Key() string {
	kind := "host"
	if r.Group != "" {
		kind = "group"
	}
	return strings.Join([]string{r.sourceName(), r.Proto, r.Port.String(), kind}, "|")
}

func (r Rule) sourceName() string {
	if r.Group != "" {
		return r.Group
	}
	return r.Host
}

// Expand turns the flat rule list into the inbound firewall for one node. Every
// rule whose destination matches the node becomes one entry. The result is
// sorted, so two calls on the same input return the same slice.
func Expand(mesh config.Mesh, node config.Node) ([]Rule, error) {
	groups := map[string]bool{}
	for _, group := range node.Groups {
		groups[group] = true
	}

	seen := map[string]Rule{}
	for _, rule := range mesh.Rules {
		if !destinesNode(rule.To, node, groups) {
			continue
		}
		source, err := sourceOf(mesh, rule.From)
		if err != nil {
			return nil, err
		}
		ports, err := parsePorts(rule.Ports)
		if err != nil {
			return nil, fmt.Errorf("rule %s -> %s: %w", rule.From, rule.To, err)
		}
		for _, port := range ports {
			entry := Rule{
				Port:  port.port,
				Proto: port.proto,
				Host:  source.host,
				Group: source.group,
			}
			seen[entry.Key()] = entry
		}
	}

	rules := make([]Rule, 0, len(seen))
	for _, rule := range seen {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Key() < rules[j].Key() })
	return rules, nil
}

// destinesNode reports whether a rule destination covers this node.
func destinesNode(to string, node config.Node, groups map[string]bool) bool {
	if to == "any" || to == node.Name {
		return true
	}
	return groups[to]
}

type source struct {
	host  string
	group string
}

// sourceOf turns a rule source into the Nebula form. A group stays a group, a
// node name becomes a host, and "any" becomes host: any.
func sourceOf(mesh config.Mesh, from string) (source, error) {
	if from == "any" {
		return source{host: "any"}, nil
	}
	for _, node := range mesh.Nodes {
		if node.Name == from {
			return source{host: from}, nil
		}
	}
	for _, node := range mesh.Nodes {
		for _, group := range node.Groups {
			if group == from {
				return source{group: from}, nil
			}
		}
	}
	return source{}, fmt.Errorf("source %q is not a node or a group", from)
}

type portSpec struct {
	port  Port
	proto string
}

// parsePorts reads entries such as 443/tcp, 8000-8100/tcp, and any/icmp.
func parsePorts(entries []string) ([]portSpec, error) {
	ports := make([]portSpec, 0, len(entries))
	for _, entry := range entries {
		port, proto, found := strings.Cut(entry, "/")
		if !found {
			return nil, fmt.Errorf("port %q has no protocol, use a form such as 443/tcp", entry)
		}
		proto = strings.ToLower(strings.TrimSpace(proto))
		port = strings.TrimSpace(port)

		switch proto {
		case "tcp", "udp", "icmp", "any":
		default:
			return nil, fmt.Errorf("port %q has protocol %q, use tcp, udp, icmp, or any", entry, proto)
		}

		if proto == "icmp" {
			ports = append(ports, portSpec{port: Port{}, proto: proto})
			continue
		}
		if port == "any" {
			ports = append(ports, portSpec{port: AnyPort(), proto: proto})
			continue
		}

		parsed, err := parsePort(port)
		if err != nil {
			return nil, fmt.Errorf("port %q: %w", entry, err)
		}
		ports = append(ports, portSpec{port: parsed, proto: proto})
	}
	return ports, nil
}

func parsePort(port string) (Port, error) {
	first, last, isRange := strings.Cut(port, "-")
	low, err := strconv.Atoi(first)
	if err != nil || low < 1 || low > 65535 {
		return Port{}, fmt.Errorf("%q is not a port number or a range", port)
	}
	if !isRange {
		return NumberPort(low), nil
	}
	high, err := strconv.Atoi(last)
	if err != nil || high < low || high > 65535 {
		return Port{}, fmt.Errorf("%q is not a port range", port)
	}
	return RangePort(port), nil
}
