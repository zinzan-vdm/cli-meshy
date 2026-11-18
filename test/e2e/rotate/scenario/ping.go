package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// delay is the pause between two waits. A variable so that a test can set it.
var delay = 500 * time.Millisecond

func sleep() { time.Sleep(delay) }

// pingLines reads the two numbers of a ping summary. Busybox and iputils both
// write "N packets transmitted, M packets received".
var pingLines = regexp.MustCompile(`(\d+) packets transmitted, (\d+) (?:packets )?received`)

// parsePing returns the replies that the ping sent and the replies that came
// back. A log without a summary is an error, because the ping may have died
// before the rotation started.
func parsePing(out string) (sent, got int, err error) {
	m := pingLines.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, fmt.Errorf("the ping log holds no summary: %q", out)
	}
	sent, err = strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, err
	}
	got, err = strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, err
	}
	return sent, got, nil
}
