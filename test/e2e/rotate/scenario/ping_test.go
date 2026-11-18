package main

import "testing"

const pingLog = `PING 10.42.0.1 (10.42.0.1): 56 data bytes
64 bytes from 10.42.0.1: seq=0 ttl=64 time=1.204 ms
64 bytes from 10.42.0.1: seq=1 ttl=64 time=1.310 ms
64 bytes from 10.42.0.1: seq=2 ttl=64 time=1.188 ms

--- 10.42.0.1 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 1.188/1.234/1.310 ms
`

const lossyLog = `--- 10.42.0.1 ping statistics ---
40 packets transmitted, 38 packets received, 5% packet loss
`

func TestParsePingReadsTheSummary(t *testing.T) {
	sent, got, err := parsePing(pingLog)
	if err != nil {
		t.Fatalf("parsePing: %v", err)
	}
	if sent != 3 || got != 3 {
		t.Errorf("sent = %d, got = %d, want 3 and 3", sent, got)
	}
}

func TestParsePingReportsLostReplies(t *testing.T) {
	sent, got, err := parsePing(lossyLog)
	if err != nil {
		t.Fatalf("parsePing: %v", err)
	}
	if sent != 40 || got != 38 {
		t.Errorf("sent = %d, got = %d, want 40 and 38", sent, got)
	}
}

func TestParsePingRefusesALogWithoutASummary(t *testing.T) {
	if _, _, err := parsePing("64 bytes from 10.42.0.1: seq=0 ttl=64 time=1.2 ms\n"); err == nil {
		t.Error("parsePing accepted a log without a summary")
	}
}
