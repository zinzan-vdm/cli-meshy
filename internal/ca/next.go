package ca

import (
	"fmt"
	"strconv"
	"strings"
)

// Next returns the name after the highest authority in names. The number only
// grows, so a rotation reads the same every time and a vault that keeps the old
// authorities never hands out a name twice.
func Next(names []string) string {
	highest := 0
	for _, name := range names {
		rest, found := strings.CutPrefix(name, "ca-")
		if !found {
			continue
		}
		number, err := strconv.Atoi(rest)
		if err != nil || number < 1 {
			continue
		}
		if number > highest {
			highest = number
		}
	}
	return fmt.Sprintf("ca-%d", highest+1)
}
