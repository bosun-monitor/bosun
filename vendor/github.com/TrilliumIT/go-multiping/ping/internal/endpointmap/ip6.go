package endpointmap

import (
	"encoding/binary"
	"net"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
	"github.com/TrilliumIT/go-multiping/ping/internal/seqmap"
)

func iToIP6Idx(ip net.IP, id ping.ID) [18]byte {
	var r [18]byte
	copy(r[0:16], ip.To16())
	binary.LittleEndian.PutUint16(r[16:], uint16(id))
	return r
}

type ip6m map[[18]byte]*seqmap.Map

func (i ip6m) add(ip net.IP, id ping.ID, sm *seqmap.Map) {
	i[toIP6Idx(ip, id)] = sm
}

func (i ip6m) del(ip net.IP, id ping.ID) {
	delete(i, toIP6Idx(ip, id))
}

func (i ip6m) get(ip net.IP, id ping.ID) (*seqmap.Map, bool) {
	sm, ok := i[toIP6Idx(ip, id)]
	return sm, ok
}

func (i ip6m) length() int {
	return len(i)
}
