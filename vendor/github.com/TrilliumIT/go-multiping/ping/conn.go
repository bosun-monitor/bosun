package ping

import (
	"net"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/conn"
	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/socket"
)

type ipConn struct {
	s       *Socket
	dst     *net.IPAddr
	id      ping.ID
	timeout time.Duration
	handle  func(*ping.Ping, error)
}

// ErrNoIDs is returned when there are no icmp ids left to use
//
// Either you are trying to ping the same host with more than 2^16 connections
// or you are on windows and are running more than 2^16 connections total
var ErrNoIDs = socket.ErrNoIDs

// ErrTimedOut is returned when a ping times out
var ErrTimedOut = socket.ErrTimedOut

func (s *Socket) newIPConn(dst *net.IPAddr, handle func(*ping.Ping, error), timeout time.Duration) (*IPConn, error) {
	c := &IPConn{
		count: -1,
	}
	var err error
	c.ipc, err = s.newipConn(dst, handle, timeout)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Socket) newipConn(dst *net.IPAddr, handle func(*ping.Ping, error), timeout time.Duration) (*ipConn, error) {
	ipc := &ipConn{
		dst:     dst,
		timeout: timeout,
		s:       s,
		handle:  handle,
	}
	var err error
	ipc.id, err = s.s.Add(dst, ipc.handle)
	return ipc, err
}

func (c *ipConn) close() error {
	if c.s == nil {
		return nil
	}
	defer func() { c.s = nil }() // make anybody who tries to send after close panic
	return c.s.s.Del(c.dst.IP, c.id)
}

func (c *ipConn) drain() {
	if c.s == nil {
		return
	}
	c.s.s.Drain(c.dst.IP, c.id)
}

// ErrNotRunning is returned if a ping is set to a closed connection.
var ErrNotRunning = conn.ErrNotRunning

func (c *ipConn) sendPing(p *ping.Ping) {
	p.Dst, p.ID, p.TimeOut = c.dst, c.id, c.timeout
	c.s.s.SendPing(p)
}
