package timeoutmap

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

func toIP4Idx(ip net.IP, id ping.ID, seq ping.Seq) [8]byte {
	var r [8]byte
	copy(r[0:4], ip.To4())
	binary.LittleEndian.PutUint16(r[4:6], uint16(id))
	binary.LittleEndian.PutUint16(r[6:], uint16(seq))
	return r
}

func fromIP4Idx(b [8]byte) (ip net.IP, id ping.ID, seq ping.Seq) {
	return net.IPv4(b[0], b[1], b[2], b[3]),
		ping.ID(binary.LittleEndian.Uint16(b[4:6])),
		ping.Seq(binary.LittleEndian.Uint16(b[6:]))
}

type ip4m map[[8]byte]time.Time

func (i ip4m) add(ip net.IP, id ping.ID, seq ping.Seq, t time.Time) {
	i[toIP4Idx(ip, id, seq)] = t
}

func (i ip4m) del(ip net.IP, id ping.ID, seq ping.Seq) {
	delete(i, toIP4Idx(ip, id, seq))
}

func (i ip4m) exists(ip net.IP, id ping.ID, seq ping.Seq) bool {
	_, ok := i[toIP4Idx(ip, id, seq)]
	return ok
}

func (i ip4m) getNext() (ip net.IP, id ping.ID, seq ping.Seq, t time.Time) {
	for k, v := range i {
		if v.Before(t) || t.IsZero() {
			t = v
			ip, id, seq = fromIP4Idx(k)
		}
	}
	return
}
