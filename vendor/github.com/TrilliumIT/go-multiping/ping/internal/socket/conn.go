package socket

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/conn"
	"github.com/TrilliumIT/go-multiping/ping/internal/endpointmap"
	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/timeoutmap"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Add adds an ip address to the socket, listening for ICMP echos to that IP.
func (s *Socket) Add(dst *net.IPAddr, h func(*ping.Ping, error)) (ping.ID, error) {
	s.l.Lock()
	defer s.l.Unlock()
	conn, em, tm, _, setCancel := s.getConnMaps(dst.IP)
	return s.add(conn, em, tm, setCancel, dst, h)
}

// ErrNoIDs is returned if there are no more avaliable ICMP IDs.
var ErrNoIDs = errors.New("no avaliable icmp IDs")

// ErrTimedOut is returned when a packet times out.
var ErrTimedOut = errors.New("timed out")

func (s *Socket) add(
	conn *conn.Conn, em *endpointmap.Map, tm *timeoutmap.Map, setCancel func(func()),
	dst *net.IPAddr, h func(*ping.Ping, error),
) (ping.ID, error) {
	var id int
	var sl int
	var err error
	startID := rand.Intn(1<<16 - 1)
	for id = startID; id < startID+1<<16-1; id++ {
		_, sl, err = em.Add(dst.IP, ping.ID(id), h)
		if err == endpointmap.ErrAlreadyExists {
			continue
		}
		if sl == 1 {
			err = conn.Run(s.Workers)
			if err != nil {
				return 0, err
			}
			ctx, cancel := context.WithCancel(context.Background())
			setCancel(cancel)
			go func() {
				for ip, id, seq, _ := tm.Next(ctx); ip != nil; ip, id, seq, _ = tm.Next(ctx) {
					handle(em, tm, &ping.Ping{Dst: &net.IPAddr{IP: ip}, ID: id, Seq: seq}, ErrTimedOut)
				}
			}()
		}
		return ping.ID(id), err
	}
	return 0, ErrNoIDs
}

// Del removes an IP from the socket, so returned echos will no longer be recieved.
func (s *Socket) Del(dst net.IP, id ping.ID) error {
	s.l.Lock()
	defer s.l.Unlock()
	conn, em, tm, cancel, _ := s.getConnMaps(dst)
	return s.del(conn, em, tm, cancel, dst, id)
}

func (s *Socket) del(
	conn *conn.Conn, em *endpointmap.Map, tm *timeoutmap.Map, cancel func(),
	dst net.IP, id ping.ID) error {
	sm, sl, err := em.Pop(dst, id)
	if err != nil {
		return err
	}
	sm.Close()
	if sl == 0 {
		err = conn.Stop()
		cancel()
	}
	return err
}

// Drain blocks until all pending pings to dst have been handled
func (s *Socket) Drain(dst net.IP, id ping.ID) {
	s.l.Lock()
	conn, em, tm, cancel, _ := s.getConnMaps(dst)
	s.drain(conn, em, tm, cancel, dst, id)
	s.l.Unlock()
}

func (s *Socket) drain(
	conn *conn.Conn, em *endpointmap.Map, tm *timeoutmap.Map, cancel func(),
	dst net.IP, id ping.ID) {
	sm, _, _ := em.Get(dst, id)
	if sm == nil {
		return
	}
	sm.Drain()
}

// SendPing sends the ping, in the process it sets the sent time
// This object will be held in the sequencemap until the reply is recieved
// or it times out, at which point it will be handled. The handled object
// will be the same as the sent ping but with the additional information from
// having been recieved.
func (s *Socket) SendPing(p *ping.Ping) {
	conn, em, tm, _, _ := s.getConnMaps(p.Dst.IP)
	sm, ok, _ := em.Get(p.Dst.IP, p.ID)
	if !ok {
		return
	}

	sl := sm.Add(p)
	if sl == 0 {
		// Sending was closed
		return
	}
	dst, id, seq, to := p.Dst.IP, p.ID, p.Seq, p.TimeOut
	if to > 0 {
		tm.Add(dst, id, seq, time.Now().Add(2*to))
	}
	tot, err := conn.Send(p)
	if err != nil {
		tm.Del(dst, id, seq)
		if rp, _, err2 := sm.Pop(seq); err2 == nil {
			sm.Handle(rp, err)
		}
		return
	}
	if to > 0 {
		// update timeout with accurate timeout time
		tm.Update(dst, id, seq, tot)
	}
}
