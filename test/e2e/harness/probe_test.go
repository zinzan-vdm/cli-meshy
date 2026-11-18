package main

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// timeoutError is the error that a dropped packet produces: the dial gave up.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// refusedError is a closed port that answers. The packet arrived, so a firewall
// rule did not drop it.
var refusedError = errors.New("connect: connection refused")

type fakeConn struct {
	net.Conn
	closed bool
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func dialReturning(err error) (DialFunc, *fakeConn) {
	conn := &fakeConn{}
	return func(string, time.Duration) (net.Conn, error) { return conn, err }, conn
}

func TestExpectOpen(t *testing.T) {
	dial, conn := dialReturning(nil)
	if err := ExpectOpen("node-b", "10.42.0.2:8080", time.Second, dial); err != nil {
		t.Fatalf("a connection that completed failed: %v", err)
	}
	if !conn.closed {
		t.Error("the harness left the connection open")
	}

	dial, _ = dialReturning(refusedError)
	err := ExpectOpen("node-b", "10.42.0.2:8080", time.Second, dial)
	if err == nil {
		t.Fatal("a refused connection passed")
	}
	if !strings.Contains(err.Error(), "node-b") || !strings.Contains(err.Error(), "10.42.0.2:8080") {
		t.Errorf("the message does not name the node and the port: %v", err)
	}
}

func TestExpectTimeout(t *testing.T) {
	dial, _ := dialReturning(timeoutError{})
	if err := ExpectTimeout("node-b", "10.42.0.2:9090", time.Second, dial); err != nil {
		t.Fatalf("a dropped packet failed: %v", err)
	}

	dial, conn := dialReturning(nil)
	err := ExpectTimeout("node-b", "10.42.0.2:9090", time.Second, dial)
	if err == nil {
		t.Fatal("a connection that completed passed")
	}
	if !strings.Contains(err.Error(), "9090") {
		t.Errorf("the message does not name the port: %v", err)
	}
	if !conn.closed {
		t.Error("the harness left the connection open")
	}

	dial, _ = dialReturning(refusedError)
	err = ExpectTimeout("node-b", "10.42.0.2:9090", time.Second, dial)
	if err == nil {
		t.Fatal("a refusal passed as a timeout")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("the message does not say what happened: %v", err)
	}
}

func TestExpectPing(t *testing.T) {
	if err := ExpectPing("node-b", "10.42.0.2", func(string) error { return nil }); err != nil {
		t.Fatalf("a reply failed: %v", err)
	}

	err := ExpectPing("node-b", "10.42.0.2", func(string) error { return errors.New("exit status 1") })
	if err == nil {
		t.Fatal("a lost reply passed")
	}
	if !strings.Contains(err.Error(), "node-b") || !strings.Contains(err.Error(), "10.42.0.2") {
		t.Errorf("the message does not name the node and the address: %v", err)
	}
}

func TestWaitStopsOnSuccess(t *testing.T) {
	calls := 0
	err := wait(time.Second, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("wait returned %v", err)
	}
	if calls != 3 {
		t.Errorf("wait called the probe %d times, want 3", calls)
	}
}

func TestWaitReturnsTheLastError(t *testing.T) {
	err := wait(time.Millisecond, time.Millisecond, func() error {
		return errors.New("still nothing on 9090")
	})
	if err == nil {
		t.Fatal("wait returned no error")
	}
	if !strings.Contains(err.Error(), "9090") {
		t.Errorf("wait lost the error that it should report: %v", err)
	}
}
