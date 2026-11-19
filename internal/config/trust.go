package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddTrust adds name to the ca.trust list in mesh.yml and leaves every other
// byte of the file as it was. The command that rotates an authority edits a file
// that a person reviews, so a comment, a blank line, and the order of the keys
// must survive the edit. A yaml round trip drops the blank lines, so this
// function edits the text of the one line or the one list that changes.
func AddTrust(dir, name string) (bool, error) {
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

	for i := start; i < end; i++ {
		line := lines[i]
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "trust:") {
			continue
		}

		edited, changed, err := addToTrust(lines, i, end, name)
		if err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
		if !changed {
			return false, nil
		}
		if err := os.WriteFile(path, []byte(strings.Join(edited, "\n")), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", path, err)
		}
		return true, nil
	}

	return false, fmt.Errorf("%s has no ca.trust list", path)
}

// caBlock returns the line range of the body of the top level ca mapping. The
// range starts after the ca line and ends before the next top level key.
func caBlock(lines []string) (int, int) {
	start := -1
	for i, line := range lines {
		if keyOf(line) == "ca" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return -1, -1
	}

	for i := start; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		return start, i
	}
	return start, len(lines)
}

// keyOf returns the top level key of a line, or an empty string for a line that
// is indented, blank, or a comment.
func keyOf(line string) string {
	if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
		return ""
	}
	if i := strings.IndexAny(line, ": #"); i >= 0 {
		return line[:i]
	}
	return line
}

// addToTrust appends name to the trust key on line i. The key holds either a
// flow list on the line of the key or a block list on the lines below it.
func addToTrust(lines []string, i, end int, name string) ([]string, bool, error) {
	line := lines[i]
	colon := strings.Index(line, "trust:")
	head := line[:colon+len("trust:")]

	pad := leadingSpace(line[colon+len("trust:"):])
	value := strings.TrimLeft(line[colon+len("trust:"):], " ")

	switch {
	case strings.HasPrefix(value, "["):
		return addToFlowList(lines, i, head, pad, value, name)
	case value == "" || strings.HasPrefix(value, "#"):
		return addToBlockList(lines, i, end, name)
	default:
		return nil, false, fmt.Errorf("ca.trust is written in a form this build cannot edit")
	}
}

// addToFlowList appends name to a list such as `trust: [ca-1]`.
func addToFlowList(lines []string, i int, head, pad, value, name string) ([]string, bool, error) {
	close := strings.Index(value, "]")
	if close < 0 {
		return nil, false, fmt.Errorf("ca.trust holds a list that has no end")
	}

	items := splitItems(value[1:close])
	q := quoteOf(items)
	for _, item := range items {
		if strings.Trim(item, `"'`) == name {
			return nil, false, nil
		}
	}
	items = append(items, q+name+q)

	edited := make([]string, len(lines))
	copy(edited, lines)
	edited[i] = head + pad + "[" + strings.Join(items, ", ") + "]" + value[close+1:]
	return edited, true, nil
}

// addToBlockList appends name to a list that starts on the line after the trust
// key, one authority per line.
func addToBlockList(lines []string, i, end int, name string) ([]string, bool, error) {
	last := -1
	prefix := ""
	for j := i + 1; j < end; j++ {
		item := lines[j]
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if strings.Trim(strings.TrimPrefix(trimmed, "- "), `"'`) == name {
			return nil, false, nil
		}
		last = j
		prefix = item[:len(item)-len(strings.TrimLeft(item, " \t"))]
	}
	if last < 0 {
		return nil, false, fmt.Errorf("ca.trust holds no authority that this build can append to")
	}

	edited := make([]string, 0, len(lines)+1)
	edited = append(edited, lines[:last+1]...)
	edited = append(edited, prefix+"- "+name)
	edited = append(edited, lines[last+1:]...)
	return edited, true, nil
}

// leadingSpace returns the spaces at the start of s.
func leadingSpace(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " "))]
}

// splitItems returns the entries of a flow list, without the spaces around them.
func splitItems(body string) []string {
	var items []string
	for _, item := range strings.Split(body, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// quoteOf returns the quote that the items carry, so a new entry keeps the style
// of the list around it.
func quoteOf(items []string) string {
	for _, item := range items {
		switch {
		case strings.HasPrefix(item, `"`):
			return `"`
		case strings.HasPrefix(item, `'`):
			return `'`
		}
	}
	return ""
}

// RemoveTrust removes name from the ca.trust list in mesh.yml and leaves every
// other byte of the file as it was. Phase 3 drops the old authority from a file
// that a person reviews, so a comment, a blank line, and the order of the keys
// must survive the edit.
func RemoveTrust(dir, name string) (bool, error) {
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

	for i := start; i < end; i++ {
		if !strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "trust:") {
			continue
		}
		edited, changed, err := removeFromTrust(lines, i, end, name)
		if err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
		if !changed {
			return false, nil
		}
		if err := os.WriteFile(path, []byte(strings.Join(edited, "\n")), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", path, err)
		}
		return true, nil
	}

	return false, fmt.Errorf("%s has no ca.trust list", path)
}

// removeFromTrust drops name from the trust key on line i. The key holds either
// a flow list on the line of the key or a block list on the lines below it.
func removeFromTrust(lines []string, i, end int, name string) ([]string, bool, error) {
	line := lines[i]
	colon := strings.Index(line, "trust:")
	head := line[:colon+len("trust:")]

	pad := leadingSpace(line[colon+len("trust:"):])
	value := strings.TrimLeft(line[colon+len("trust:"):], " ")

	switch {
	case strings.HasPrefix(value, "["):
		return removeFromFlowList(lines, i, head, pad, value, name)
	case value == "" || strings.HasPrefix(value, "#"):
		return removeFromBlockList(lines, i, end, name)
	default:
		return nil, false, fmt.Errorf("ca.trust is written in a form this build cannot edit")
	}
}

// removeFromFlowList drops name from a list such as `trust: [ca-1, ca-2]`.
func removeFromFlowList(lines []string, i int, head, pad, value, name string) ([]string, bool, error) {
	close := strings.Index(value, "]")
	if close < 0 {
		return nil, false, fmt.Errorf("ca.trust holds a list that has no end")
	}

	items := splitItems(value[1:close])
	kept := make([]string, 0, len(items))
	found := false
	for _, item := range items {
		if strings.Trim(item, `"'`) == name {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if !found {
		return nil, false, nil
	}

	edited := make([]string, len(lines))
	copy(edited, lines)
	edited[i] = head + pad + "[" + strings.Join(kept, ", ") + "]" + value[close+1:]
	return edited, true, nil
}

// removeFromBlockList drops the line of name from a list that starts on the
// line after the trust key, one authority per line.
func removeFromBlockList(lines []string, i, end int, name string) ([]string, bool, error) {
	for j := i + 1; j < end; j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		if strings.Trim(strings.TrimPrefix(trimmed, "- "), `"'`) != name {
			continue
		}
		edited := make([]string, 0, len(lines)-1)
		edited = append(edited, lines[:j]...)
		edited = append(edited, lines[j+1:]...)
		return edited, true, nil
	}
	return nil, false, nil
}
