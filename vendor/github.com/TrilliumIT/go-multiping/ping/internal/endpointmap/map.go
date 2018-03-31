package endpointmap

import (
	"errors"
	"net"
	"sync"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/seqmap"
)

// Map holds an endpoint map mapping endpoints to sequence maps
type Map struct {
	l sync.RWMutex
	m iMap
}

// New creates a new endpoint map
func New(proto int) *Map {
	m := &Map{}
	switch proto {
	case 4:
		m.m = make(ip4m)
	case 6:
		m.m = make(ip6m)
	default:
		panic("invalid protocol")
	}
	return m
}

type iMap interface {
	add(net.IP, ping.ID, *seqmap.Map)
	del(net.IP, ping.ID)
	get(net.IP, ping.ID) (*seqmap.Map, bool)
	length() int
}

// ErrAlreadyExists is returned if you attempt to add an endpoint to the map that already exists.
var ErrAlreadyExists = errors.New("already exists")

// ErrDoesNotExist is returned if you try to get an endpoint that does not exist
var ErrDoesNotExist = errors.New("does not exist")

// Add returns the length of the map after modification
func (m *Map) Add(ip net.IP, id ping.ID, h func(*ping.Ping, error)) (sm *seqmap.Map, l int, err error) {
	var ok bool
	m.l.Lock()
	sm, ok = m.m.get(ip, id)
	if ok {
		err = ErrAlreadyExists
	} else {
		sm = seqmap.New(h)
		m.m.add(ip, id, sm)
	}
	l = m.m.length()
	m.l.Unlock()
	return sm, l, err
}

// Pop removes and returns a sequence map
func (m *Map) Pop(ip net.IP, id ping.ID) (sm *seqmap.Map, l int, err error) {
	var ok bool
	m.l.Lock()
	sm, ok = m.m.get(ip, id)
	if !ok {
		err = ErrDoesNotExist
	}
	m.m.del(ip, id)
	l = m.m.length()
	m.l.Unlock()
	return sm, l, err
}

// Get gets a sequence map
func (m *Map) Get(ip net.IP, id ping.ID) (sm *seqmap.Map, ok bool, length int) {
	m.l.RLock()
	sm, ok = m.m.get(ip, id)
	length = m.m.length()
	m.l.RUnlock()
	return sm, ok, length
}
