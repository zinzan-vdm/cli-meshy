package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SetAuthority adds an authority to the ca block of mesh.yml when the file does
// not name it yet. The file is rewritten only when something is missing, so a
// mesh that already trusts its authority keeps its bytes and its comments.
func SetAuthority(dir, name string) (bool, error) {
	path := filepath.Join(dir, FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s is not a mapping", path)
	}

	root := doc.Content[0]
	caNode := mappingValue(root, "ca")
	if caNode == nil {
		caNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content, scalar("ca"), caNode)
	}
	if caNode.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: ca is not a mapping", path)
	}

	changed := false
	if trust := mappingValue(caNode, "trust"); trust == nil {
		caNode.Content = append(caNode.Content, scalar("trust"), &yaml.Node{
			Kind:    yaml.SequenceNode,
			Tag:     "!!seq",
			Style:   yaml.FlowStyle,
			Content: []*yaml.Node{scalar(name)},
		})
		changed = true
	} else if !hasScalar(trust, name) {
		trust.Content = append(trust.Content, scalar(name))
		changed = true
	}

	if active := mappingValue(caNode, "active"); active == nil {
		caNode.Content = append(caNode.Content, scalar("active"), scalar(name))
		changed = true
	}

	if !changed {
		return false, nil
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return false, err
	}
	if err := encoder.Close(); err != nil {
		return false, err
	}

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}

	return true, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func hasScalar(node *yaml.Node, value string) bool {
	for _, item := range node.Content {
		if item.Value == value {
			return true
		}
	}
	return false
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
