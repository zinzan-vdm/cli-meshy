package sshtest

import (
	"io"
	"net"
	"sync"
	"time"
)

// memPipe returns two ends of one connection that holds what is written in a
// buffer. net.Pipe has no buffer, and the SSH handshake writes its version from
// both ends at the same time, so a net.Pipe pair deadlocks before the first
// packet.
func memPipe() (net.Conn, net.Conn) {
	a := &memConn{closed: make(chan struct{})}
	b := &memConn{closed: make(chan struct{})}
	a.in = make(chan []byte, 1024)
	b.in = make(chan []byte, 1024)
	a.out = b.in
	b.out = a.in
	return a, b
}

type memConn struct {
	in     chan []byte
	out    chan []byte
	buf    []byte
	closed chan struct{}
	once   sync.Once
}

func (c *memConn) Read(p []byte) (int, error) {
	for len(c.buf) == 0 {
		select {
		case data, ok := <-c.in:
			if !ok {
				return 0, io.EOF
			}
			c.buf = data
		case <-c.closed:
			return 0, net.ErrClosed
		}
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

func (c *memConn) Write(p []byte) (int, error) {
	select {
	case c.out <- append([]byte(nil), p...):
		return len(p), nil
	case <-c.closed:
		return 0, net.ErrClosed
	}
}

func (c *memConn) Close() error {
	c.once.Do(func() {
		close(c.closed)
		close(c.out)
	})
	return nil
}

func (c *memConn) LocalAddr() net.Addr                { return pipeAddr{} }
func (c *memConn) RemoteAddr() net.Addr               { return pipeAddr{} }
func (c *memConn) SetDeadline(_ time.Time) error      { return nil }
func (c *memConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *memConn) SetWriteDeadline(_ time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
