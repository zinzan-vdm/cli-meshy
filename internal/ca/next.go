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
	return fmt.Sprintf("ca-%d", highest(names)+1)
}

// NewActive returns the trusted authority that signs new certificates after a
// rotation. It is the highest numbered authority in names, and it must outrank
// active. An empty result means that the active authority is already the
// highest, so the rotation has nothing left to sign.
func NewActive(names []string, active string) string {
	top := highest(names)
	if top <= number(active) {
		return ""
	}
	return fmt.Sprintf("ca-%d", top)
}

// highest returns the number of the highest named authority, or zero when the
// list holds none.
func highest(names []string) int {
	top := 0
	for _, name := range names {
		if n := number(name); n > top {
			top = n
		}
	}
	return top
}

// number returns the number in a name such as ca-2, or zero for a name that
// carries none.
func number(name string) int {
	rest, found := strings.CutPrefix(name, "ca-")
	if !found {
		return 0
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 {
		return 0
	}
	return n
}
