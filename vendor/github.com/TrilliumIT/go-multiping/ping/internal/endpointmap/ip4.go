package endpointmap

import (
	"encoding/binary"
	"net"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/seqmap"
)

func iToIP4Idx(ip net.IP, id ping.ID) [6]byte {
	var r [6]byte
	copy(r[0:4], ip.To4())
	binary.LittleEndian.PutUint16(r[4:], uint16(id))
	return r
}

type ip4m map[[6]byte]*seqmap.Map

func (i ip4m) add(ip net.IP, id ping.ID, sm *seqmap.Map) {
	i[toIP4Idx(ip, id)] = sm
}

func (i ip4m) del(ip net.IP, id ping.ID) {
	delete(i, toIP4Idx(ip, id))
}

func (i ip4m) get(ip net.IP, id ping.ID) (*seqmap.Map, bool) {
	sm, ok := i[toIP4Idx(ip, id)]
	return sm, ok
}

func (i ip4m) length() int {
	return len(i)
}
