package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetActive names the authority that signs new certificates in mesh.yml and
// leaves every other byte of the file as it was. A YAML round trip drops the
// blank lines between the blocks, so this function edits the one line that
// changes. See docs/patterns/0020.
func SetActive(dir, name string) (bool, error) {
	path := filepath.Join(dir, FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	lines := strings.Split(string(body), "\n")
	start, end := caBlock(lines)
	if start < 0 {
		return false, fmt.Errorf("%s has no ca block", path)
	}

	edited := lines
	for i := start; i < end; i++ {
		if !strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "active:") {
			continue
		}
		line, changed := setActiveLine(lines[i], name)
		if !changed {
			return false, nil
		}
		edited = make([]string, len(lines))
		copy(edited, lines)
		edited[i] = line
		return writeMesh(path, edited)
	}

	// A file that names no active authority takes the default of ca-1, so the
	// line has to appear before the file can name another one.
	inserted, err := insertActive(lines, start, end, name)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return writeMesh(path, inserted)
}

func writeMesh(path string, lines []string) (bool, error) {
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// setActiveLine replaces the value of an active line and keeps its indent, its
// spacing, its quotes, and its comment.
func setActiveLine(line, name string) (string, bool) {
	colon := strings.Index(line, "active:")
	head := line[:colon+len("active:")]
	rest := line[colon+len("active:"):]

	pad := leadingSpace(rest)
	after := rest[len(pad):]

	comment := ""
	if i := strings.Index(after, "#"); i >= 0 {
		comment = after[i:]
		after = after[:i]
	}
	end := len(strings.TrimRight(after, " \t"))
	value, gap := after[:end], after[end:]

	quote, bare := quotesOf(value)
	if bare == name {
		return line, false
	}
	return head + pad + quote + name + quote + gap + comment, true
}

// quotesOf returns the quotes around a value and the value without them.
func quotesOf(value string) (string, string) {
	for _, quote := range []string{`"`, `'`} {
		if strings.HasPrefix(value, quote) && strings.HasSuffix(value, quote) && len(value) >= 2 {
			return quote, value[1 : len(value)-1]
		}
	}
	return "", value
}

// insertActive writes an active line after the trust key, or after the last
// authority of a block list, at the indent of the trust key.
func insertActive(lines []string, start, end int, name string) ([]string, error) {
	for i := start; i < end; i++ {
		trimmed := strings.TrimLeft(lines[i], " 	")
		if !strings.HasPrefix(trimmed, "trust:") {
			continue
		}
		indent := leadingSpace(lines[i])
		at := i
		// A block list holds its entries on the lines below the key, so the new
		// line belongs after the last entry and not in the middle of the list.
		if strings.TrimSpace(trimmed[len("trust:"):]) == "" {
			for j := i + 1; j < end; j++ {
				if !strings.HasPrefix(strings.TrimSpace(lines[j]), "- ") {
					break
				}
				at = j
			}
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:at+1]...)
		out = append(out, indent+"active: "+name)
		out = append(out, lines[at+1:]...)
		return out, nil
	}
	return nil, fmt.Errorf("ca block holds no trust list to follow")
}
