// Package installers renders the three installers that a bundle carries.
package installers

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.tmpl
var templates embed.FS

// Values are the paths, names, and release facts that an installer writes.
type Values struct {
	Mesh        string
	Node        string
	Interface   string
	Dir         string
	ConfigPath  string
	Unit        string
	UnitFile    string
	Plist       string
	BinaryPath  string
	Binary      string
	Version     string
	Asset       string
	Checksum    string
	BaseURL     string
	Destination string
}

// Names are the files that Render returns.
func Names() []string { return []string{"install.command", "install.ps1", "install.sh"} }

// Render returns each installer by the name it carries inside the archive. It
// takes values and returns bytes, so a bundle can be built with no file system.
func Render(v Values) (map[string][]byte, error) {
	out := make(map[string][]byte, len(Names()))
	for _, name := range Names() {
		tmpl, err := templates.ReadFile(name + ".tmpl")
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		parsed, err := template.New(name).Parse(string(tmpl))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		var buf bytes.Buffer
		if err := parsed.Execute(&buf, v); err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}
		out[name] = buf.Bytes()
	}
	return out, nil
}
