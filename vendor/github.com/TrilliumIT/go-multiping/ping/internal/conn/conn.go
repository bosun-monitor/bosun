package conn

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

// Conn holds a connection, either ipv4 or ipv6
type Conn struct {
	l       sync.RWMutex
	cancel  func()
	conn    conn
	handler func(*ping.Ping, error)
	proto   int
	// I had a waitgroup for workers, but it has been removed
	// There's no reason to delay a stop waiting for these to shutdown
	// If a new listen comes in, a new listener will be created
	// Just cancel the context and let them die on their own.
}

type conn interface {
	start() error
	writeTo([]byte, *net.IPAddr) (int, error)
	read() (*ping.Ping, error)
	close() error
}

// New returns a new Conn
func New(proto int, h func(*ping.Ping, error)) *Conn {
	return &Conn{
		cancel:  func() {},
		handler: h,
		proto:   proto,
	}
}

// Run runs the workers for the Conn
func (c *Conn) Run(workers int) error {
	c.l.Lock()
	if c.conn != nil {
		c.l.Unlock()
		return nil
	}

	c.conn = newConn(c.proto)
	err := c.conn.start()
	if err != nil {
		_ = c.conn.close()
		c.l.Unlock()
		return err
	}
	var ctx context.Context
	ctx, c.cancel = context.WithCancel(context.Background())
	c.runWorkers(ctx, workers, c.conn.read, c.handler)
	c.l.Unlock()
	return nil
}

// Stop stops the Conn
func (c *Conn) Stop() error {
	c.l.Lock()
	c.cancel()
	err := c.conn.close()
	if err != nil {
		c.l.Unlock()
		return err
	}
	c.conn = nil
	c.l.Unlock()
	return nil
}

// ErrNotRunning is returned if a ping is sent through a connection that is not running
var ErrNotRunning = errors.New("not running")

// Send sends a ping through a Conn
func (c *Conn) Send(p *ping.Ping) (time.Time, error) {
	c.l.RLock()
	if c.conn == nil {
		c.l.RUnlock()
		return time.Time{}, ErrNotRunning
	}

	var err error
	var b []byte
	var toT time.Time
send:
	for {
		p.Sent = time.Now()
		toT = p.TimeOutTime()
		b, err = p.ToICMPMsg()
		if err != nil {
			return toT, err
		}
		p.Len = len(b)
		_, err = c.conn.writeTo(b, p.Dst)
		if err != nil {
			subErr := err
			for {
				switch subErr.(type) {
				case syscall.Errno:
					if subErr == syscall.ENOBUFS {
						continue send
					}
					break send
				case *net.OpError:
					subErr = subErr.(*net.OpError).Err
				case *os.SyscallError:
					subErr = subErr.(*os.SyscallError).Err
				default:
					break send
				}
			}
		}
		break
	}

	c.l.RUnlock()
	return toT, err
}
