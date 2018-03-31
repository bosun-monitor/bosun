package ping

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

// HostConn is an ICMP connection based on hostname
//
// Pings run from a HostConn can be configured to periodically re-resolve
type HostConn struct {
	s              *Socket
	ipc            *ipConn
	draining       []*ipConn
	drainWg        sync.WaitGroup
	host           string
	count          int64
	reResolveEvery int
	handle         func(*ping.Ping, error)
	timeout        time.Duration
}

// NewHostConn returns a new HostConn
func NewHostConn(host string, reResolveEvery int, handle HandleFunc, timeout time.Duration) *HostConn {
	return DefaultSocket().NewHostConn(host, reResolveEvery, handle, timeout)
}

// NewHostConn returns a new HostConn
func (s *Socket) NewHostConn(host string, reResolveEvery int, handle HandleFunc, timeout time.Duration) *HostConn {
	return s.newHostConn(host, reResolveEvery, iHandle(handle), timeout)
}

func (s *Socket) newHostConn(host string, reResolveEvery int, handle func(*ping.Ping, error), timeout time.Duration) *HostConn {
	return &HostConn{
		s:              s,
		host:           host,
		reResolveEvery: reResolveEvery,
		handle:         handle,
		timeout:        timeout,
		count:          -1,
	}
}

func (h *HostConn) getNextPing() (*ping.Ping, error) {
	p := &ping.Ping{
		Count:   int(atomic.AddInt64(&h.count, 1)),
		Host:    h.host,
		TimeOut: h.timeout,
		Sent:    time.Now(),
	}
	if h.ipc == nil || (h.reResolveEvery != 0 && p.Count%h.reResolveEvery == 0) {
		var dst *net.IPAddr
		dst, err := net.ResolveIPAddr("ip", h.host)
		changed := dst == nil || h.ipc == nil || h.ipc.dst == nil || !dst.IP.Equal(h.ipc.dst.IP)
		if err != nil {
			p.Sent = time.Now()
			return p, err
		}
		if changed {
			if h.ipc != nil {
				h.drainWg.Add(1)
				go func() {
					h.ipc.drain()
					h.drainWg.Done()
				}()
				h.draining = append(h.draining, h.ipc)
			}
			h.ipc, err = h.s.newipConn(dst, h.handle, h.timeout)
			if err != nil {
				p.Sent = time.Now()
				return p, err
			}
		}
	}
	p.Sent = time.Now()
	return p, nil
}

func (h *HostConn) sendPing(p *ping.Ping, err error) {
	if err != nil {
		h.handle(p, err)
		return
	}
	h.ipc.sendPing(p)
}

// SendPing sends a ping
func (h *HostConn) SendPing() {
	h.sendPing(h.getNextPing())
}

// Close closes the host connection. Further attempts to send pings via this connection will panic.
func (h *HostConn) Close() error {
	for _, ipc := range h.draining {
		_ = ipc.close()
	}
	if h.ipc == nil {
		return nil
	}
	return h.ipc.close()
}

// Drain will block until all pending pings have been handled, either by reply or timeout
func (h *HostConn) Drain() {
	if h.ipc != nil {
		h.ipc.drain()
	}
	h.drainWg.Wait()
}

// HostOnce performs HostOnce on the default socket.
func HostOnce(host string, timeout time.Duration) (*Ping, error) {
	return DefaultSocket().HostOnce(host, timeout)
}

// HostOnce sends a single echo request and returns, it blocks until a reply is recieved or the ping times out
//
// Zero is no timeout and IPOnce will block forever if a reply is never recieved
//
// It is not recommended to use IPOnce in a loop, use Interval, or create a Conn and call SendPing() in a loop
func (s *Socket) HostOnce(host string, timeout time.Duration) (*Ping, error) {
	sendGet := func(hdl HandleFunc) (func(), func() error, error) {
		h := s.NewHostConn(host, 1, hdl, timeout)
		return h.SendPing, h.Close, nil
	}
	return runOnce(sendGet)
}

// HostInterval performs HostInterval using the default socket.
func HostInterval(ctx context.Context, host string, reResolveEvery int, handler HandleFunc, count int, interval, timeout time.Duration) error {
	return DefaultSocket().HostInterval(ctx, host, reResolveEvery, handler, count, interval, timeout)
}

// HostInterval sends a ping each interval up to count pings or until ctx is canceled.
//
// If an interval of zero is specified, it will send pings as fast as possible.
// When there are 2^16 pending pings which have not received a reply, or timed out
// sending will block. This may be a limiting factor in how quickly pings can be sent.
//
// If a timeout of zero is specifed, pings will never time out.
//
// If a count of zero is specified, interval will continue to send pings until ctx is canceled.
func (s *Socket) HostInterval(ctx context.Context, host string, reResolveEvery int, handler HandleFunc, count int, interval, timeout time.Duration) error {
	h := s.NewHostConn(host, reResolveEvery, handler, timeout)

	runInterval(ctx, h.getNextPing, h.sendPing, count, interval)
	h.Drain()
	return h.Close()
}

// HostFlood performs HostFlood using the default socket.
func HostFlood(ctx context.Context, host string, reResolveEvery int, handler HandleFunc, count int, timeout time.Duration) error {
	return DefaultSocket().HostFlood(ctx, host, reResolveEvery, handler, count, timeout)
}

// HostFlood works like HostInterval, but instead of sending on an interval, the next ping is sent as soon as the previous ping is handled.
func (s *Socket) HostFlood(ctx context.Context, host string, reResolveEvery int, handler HandleFunc, count int, timeout time.Duration) error {
	fC := make(chan struct{})
	floodHander := func(p *Ping, err error) {
		fC <- struct{}{}
		handler(p, err)
	}

	h := s.NewHostConn(host, reResolveEvery, floodHander, timeout)

	runFlood(ctx, h.getNextPing, h.sendPing, fC, count)
	h.Drain()
	return h.Close()
}
