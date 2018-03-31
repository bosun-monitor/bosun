package seqmap

import (
	"errors"
	"sync"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

// Map holds a sequence map. A sequence map the sent ping object
// indexed by the ICMP Sequence.
type Map struct {
	l            sync.RWMutex
	m            map[ping.Seq]*ping.Ping
	handle       func(*ping.Ping, error)
	seqOffset    int
	fullWaiting  bool
	unfullNotify chan struct{}
	wg           sync.WaitGroup
}

// New returns a new sequence map
func New(h func(*ping.Ping, error)) *Map {
	return &Map{
		m:            make(map[ping.Seq]*ping.Ping),
		handle:       h,
		unfullNotify: make(chan struct{}),
	}
}

// Handle is a wrapper for the upstream handler. It decrements the
// waitgroup when the upstream handler is done, providing the drain capability.
func (m *Map) Handle(p *ping.Ping, err error) {
	m.handle(p, err)
	m.wg.Done()
}

// ErrDoesNotExist is returned if you attempt to pop a sequence that does not exist.
var ErrDoesNotExist = errors.New("does not exist")

// Add adds a sequence to the seqmap
func (m *Map) Add(p *ping.Ping) (length int) {
	var idx ping.Seq
	m.l.Lock()
	for {
		if len(m.m) >= 1<<16 {
			m.fullWaiting = true
			m.l.Unlock()
			_, open := <-m.unfullNotify
			if !open {
				return 0
			}
			m.l.Lock()
			continue
		}
		idx = ping.Seq(p.Count + m.seqOffset)
		_, ok := m.m[idx]
		if ok {
			m.seqOffset++
			continue
		}
		p.Seq = idx
		m.m[idx] = p
		length = len(m.m)
		m.wg.Add(1) // wg.done is called after handler is run, so handler must be run for every pop
		break
	}
	m.l.Unlock()
	return length
}

// Pop removes and returns a ping from the seq map
func (m *Map) Pop(seq ping.Seq) (*ping.Ping, int, error) {
	idx := seq
	var l int
	var err error
	m.l.Lock()
	p, ok := m.m[idx]
	if !ok {
		err = ErrDoesNotExist
	}
	delete(m.m, idx)
	l = len(m.m)
	if m.fullWaiting && l < 1<<16 {
		m.fullWaiting = false
		m.unfullNotify <- struct{}{}
	}
	m.l.Unlock()
	return p, l, err
}

// Close is called when a connection is closed, unblocking any blocked
// sends that were waiting on a free sequence number.
func (m *Map) Close() {
	m.l.Lock()
	m.fullWaiting = false
	close(m.unfullNotify)
	m.l.Unlock()
}

// Drain waitgs on any pending pings
func (m *Map) Drain() {
	m.wg.Wait()
}
