package ping

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

// IPConn holds a connection to a destination ip address
type IPConn struct {
	count int64
	ipc   *ipConn
}

// NewIPConn creates a new connection
func NewIPConn(dst *net.IPAddr, handle HandleFunc, timeout time.Duration) (*IPConn, error) {
	return DefaultSocket().NewIPConn(dst, handle, timeout)
}

// NewIPConn creates a new connection
func (s *Socket) NewIPConn(dst *net.IPAddr, handle HandleFunc, timeout time.Duration) (*IPConn, error) {
	return s.newIPConn(dst, iHandle(handle), timeout)
}

// ID returns the ICMP ID associated with this connection
func (c *IPConn) ID() int {
	return int(c.ipc.id)
}

func (c *IPConn) getNextPing() (*ping.Ping, error) {
	p := &ping.Ping{
		Count: int(atomic.AddInt64(&c.count, 1)),
		Sent:  time.Now(),
	}
	return p, nil
}

func (c *IPConn) sendPing(p *ping.Ping, err error) {
	if err != nil {
		c.ipc.handle(p, err)
		return
	}
	c.ipc.sendPing(p)
}

// SendPing sends a ping.
//
// Errors sending will be sent to the handler.
func (c *IPConn) SendPing() {
	c.sendPing(c.getNextPing())
}

// Close closes an IPConn. SendPing after Close will panic.
func (c *IPConn) Close() error {
	return c.ipc.close()
}

// Drain blocks until all outstanding pings have been handled.
func (c *IPConn) Drain() {
	c.ipc.drain()
}

// IPOnce performs IPOnce on the default socket.
func IPOnce(dst *net.IPAddr, timeout time.Duration) (*Ping, error) {
	return DefaultSocket().IPOnce(dst, timeout)
}

// IPOnce sends a single echo request and returns, it blocks until a reply is recieved or the ping times out
//
// Zero is no timeout and IPOnce will block forever if a reply is never recieved
//
// It is not recommended to use IPOnce in a loop, use Interval, or create a Conn and call SendPing() in a loop
func (s *Socket) IPOnce(dst *net.IPAddr, timeout time.Duration) (*Ping, error) {
	sendGet := func(h HandleFunc) (func(), func() error, error) {
		c, err := s.NewIPConn(dst, h, timeout)
		return c.SendPing, c.Close, err
	}
	return runOnce(sendGet)
}

// IPInterval performs IPInterval using the default socket
func IPInterval(ctx context.Context, dst *net.IPAddr, handler HandleFunc, count int, interval, timeout time.Duration) error {
	return DefaultSocket().IPInterval(ctx, dst, handler, count, interval, timeout)
}

// IPInterval sends a ping each interval up to count pings or until ctx is canceled.
//
// If an interval of zero is specified, it will send pings as fast as possible.
// When there are 2^16 pending pings which have not received a reply, or timed out
// sending will block. This may be a limiting factor in how quickly pings can be sent.
//
// If a timeout of zero is specifed, pings will never time out.
//
// If a count of zero is specified, interval will continue to send pings until ctx is canceled.
func (s *Socket) IPInterval(ctx context.Context, dst *net.IPAddr, handler HandleFunc, count int, interval, timeout time.Duration) error {
	c, err := s.NewIPConn(dst, handler, timeout)
	if err != nil {
		return err
	}

	runInterval(ctx, c.getNextPing, c.sendPing, count, interval)
	c.Drain()
	return c.Close()
}

// IPFlood performs IPFlood using the default socket.
func IPFlood(ctx context.Context, dst *net.IPAddr, handler HandleFunc, count int, timeout time.Duration) error {
	return DefaultSocket().IPFlood(ctx, dst, handler, count, timeout)
}

// IPFlood continuously sends pings, sending the next ping as soon as the previous one is replied or times out.
func (s *Socket) IPFlood(ctx context.Context, dst *net.IPAddr, handler HandleFunc, count int, timeout time.Duration) error {
	fC := make(chan struct{})
	floodHander := func(p *Ping, err error) {
		fC <- struct{}{}
		handler(p, err)
	}

	c, err := s.NewIPConn(dst, floodHander, timeout)
	if err != nil {
		return err
	}

	runFlood(ctx, c.getNextPing, c.sendPing, fC, count)
	c.Drain()
	return c.Close()
}
