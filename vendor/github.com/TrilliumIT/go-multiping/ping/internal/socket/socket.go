package socket

import (
	"net"
	"sync"

	"github.com/TrilliumIT/go-multiping/ping/internal/conn"
	"github.com/TrilliumIT/go-multiping/ping/internal/endpointmap"
	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/seqmap"
	"github.com/TrilliumIT/go-multiping/ping/internal/timeoutmap"
)

// Socket holds a raw socket connection, one for ipv4 and one for ipv6
type Socket struct {
	Workers int
	l       sync.RWMutex

	v4conn     *conn.Conn
	v4em       *endpointmap.Map
	v4tm       *timeoutmap.Map
	v4tmCancel func()

	v6conn     *conn.Conn
	v6em       *endpointmap.Map
	v6tm       *timeoutmap.Map
	v6tmCancel func()
}

// New creates a new socket
func New() *Socket {
	s := &Socket{
		Workers: 1,

		v4em:       endpointmap.New(4),
		v4tm:       timeoutmap.New(4),
		v4tmCancel: func() {},

		v6em:       endpointmap.New(6),
		v6tm:       timeoutmap.New(6),
		v6tmCancel: func() {},
	}
	s.v4conn = conn.New(4, s.v4handle)
	s.v6conn = conn.New(6, s.v6handle)
	return s
}

func (s *Socket) getConnMaps(ip net.IP) (
	*conn.Conn, *endpointmap.Map, *timeoutmap.Map, func(), func(func()),
) {
	if ip.To4() == nil && ip.To16() != nil {
		return s.v6conn, s.v6em, s.v6tm, s.v6tmCancel, func(f func()) { s.v6tmCancel = f }
	}
	return s.v4conn, s.v4em, s.v4tm, s.v4tmCancel, func(f func()) { s.v4tmCancel = f }
}

func handle(
	em *endpointmap.Map, tm *timeoutmap.Map,
	rp *ping.Ping, err error,
) {
	tm.Del(rp.Dst.IP, rp.ID, rp.Seq)
	sm, ok, _ := em.Get(rp.Dst.IP, rp.ID)
	if !ok {
		return
	}
	sp, _, popErr := sm.Pop(rp.Seq)
	if popErr == seqmap.ErrDoesNotExist {
		return
	}
	sp.UpdateFrom(rp)
	sm.Handle(sp, err)
}

func (s *Socket) v4handle(rp *ping.Ping, err error) {
	handle(s.v4em, s.v4tm, rp, err)
}

func (s *Socket) v6handle(rp *ping.Ping, err error) {
	handle(s.v6em, s.v6tm, rp, err)
}
