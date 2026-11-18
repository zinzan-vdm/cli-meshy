package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// DialFunc opens one TCP connection. The harness passes net.DialTimeout, and a
// test passes a fake.
type DialFunc func(addr string, timeout time.Duration) (net.Conn, error)

// PingFunc sends one echo request. The harness passes a command, and a test
// passes a fake.
type PingFunc func(addr string) error

// ExpectOpen reports a connection that failed. The message names the node and
// the address, so a broken rule names the port that it broke.
func ExpectOpen(node, addr string, timeout time.Duration, dial DialFunc) error {
	conn, err := dial(addr, timeout)
	if err != nil {
		return fmt.Errorf("node %s cannot reach %s: %w", node, addr, err)
	}
	if conn != nil {
		conn.Close()
	}
	return nil
}

// ExpectTimeout reports a connection that completed, or that failed with an
// error other than a timeout. A firewall that denies a port drops the packet,
// so a refusal means the rule is not what the mesh asks for.
func ExpectTimeout(node, addr string, timeout time.Duration, dial DialFunc) error {
	conn, err := dial(addr, timeout)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		return fmt.Errorf("node %s reached %s, the rule did not deny it", node, addr)
	}
	if !timedOut(err) {
		return fmt.Errorf("node %s expected %s to time out, got %v", node, addr, err)
	}
	return nil
}

// ExpectPing reports an echo request that failed, with the node and the address
// that it tried.
func ExpectPing(node, addr string, ping PingFunc) error {
	if err := ping(addr); err != nil {
		return fmt.Errorf("node %s cannot ping %s: %w", node, addr, err)
	}
	return nil
}

func timedOut(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// wait calls try until it succeeds or the limit runs out. It returns the last
// error, so a timeout names the thing that never happened.
func wait(limit, step time.Duration, try func() error) error {
	deadline := time.Now().Add(limit)
	for {
		err := try()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(step)
	}
}
