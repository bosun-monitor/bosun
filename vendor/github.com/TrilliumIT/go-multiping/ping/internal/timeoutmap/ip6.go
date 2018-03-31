package timeoutmap

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

func toIP6Idx(ip net.IP, id ping.ID, seq ping.Seq) [20]byte {
	var r [20]byte
	copy(r[0:16], ip.To16())
	binary.LittleEndian.PutUint16(r[16:18], uint16(id))
	binary.LittleEndian.PutUint16(r[18:], uint16(seq))
	return r
}

func fromIP6Idx(b [20]byte) (ip net.IP, id ping.ID, seq ping.Seq) {
	r := net.IP{}
	copy(r, b[0:16])
	return r,
		ping.ID(binary.LittleEndian.Uint16(b[16:18])),
		ping.Seq(binary.LittleEndian.Uint16(b[18:]))
}

type ip6m map[[20]byte]time.Time

func (i ip6m) add(ip net.IP, id ping.ID, seq ping.Seq, t time.Time) {
	i[toIP6Idx(ip, id, seq)] = t
}

func (i ip6m) del(ip net.IP, id ping.ID, seq ping.Seq) {
	delete(i, toIP6Idx(ip, id, seq))
}

func (i ip6m) exists(ip net.IP, id ping.ID, seq ping.Seq) bool {
	_, ok := i[toIP6Idx(ip, id, seq)]
	return ok
}

func (i ip6m) getNext() (ip net.IP, id ping.ID, seq ping.Seq, t time.Time) {
	for k, v := range i {
		if v.Before(t) || t.IsZero() {
			t = v
			ip, id, seq = fromIP6Idx(k)
		}
	}
	return
}
