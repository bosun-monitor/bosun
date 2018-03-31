package ping

import (
	"encoding/binary"
	"errors"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// TimeSliceLength is the length of the icmp payload holding the timestamp
	TimeSliceLength = 8
)

// see https://godoc.org/golang.org/x/net/ping/internal/iana#pkg-constants
const (
	ProtocolICMP     = 1
	ProtocolIPv6ICMP = 58 // ICMP for IPv6
)

// Seq is an ICMP Sequence
type Seq uint16

// ID is an ICMP ID
type ID uint16

// Ping is an ICMP packet that has been received
type Ping struct {
	// Host is the hostname that was pinged
	Host string
	// Src is the source IP. This is probably 0.0.0.0 for sent packets, but a
	// specific IP on the sending host for recieved packets
	Src *net.IPAddr
	// Dst is the destination IP.
	// This will be nil for recieved packets on windows. The reason is that
	// the recieve function does not provide the source address
	// on windows ICMP messages are mathed only by the 16 bit ICMP id.
	Dst *net.IPAddr
	// Count is the ICMP count
	Count int
	// ID is the ICMP ID
	ID ID
	// Seq is the ICMP Sequence
	Seq Seq
	// Sent is the time the echo was sent
	Sent time.Time
	// Recieved is the time the echo was recieved.
	Recieved time.Time
	// TimeOut is timeout duration
	TimeOut time.Duration
	// TTL is the ttl on the recieved packet.
	// This is not supported on windows and will always be zero
	TTL int
	// Len is the length of the recieved packet
	Len int
}

// UpdateFrom is for updating a sent ping with attributes from a recieved ping
func (p *Ping) UpdateFrom(rp *Ping) {
	if rp == nil || p == nil {
		return
	}

	if p.Host == "" {
		p.Host = rp.Host
	}

	if p.Src == nil && rp.Src != nil {
		p.Src = rp.Src
	}

	if p.Src != nil && rp.Src != nil {
		if p.Src.IP.IsUnspecified() && !rp.Src.IP.IsUnspecified() {
			p.Src = rp.Src
		}
	}

	if p.Dst == nil && rp.Dst != nil {
		p.Dst = rp.Dst
	}

	if p.Dst != nil && rp.Dst != nil {
		if p.Dst.IP.IsUnspecified() && !rp.Dst.IP.IsUnspecified() {
			p.Dst = rp.Dst
		}
	}

	if p.ID == 0 {
		p.ID = rp.ID
	}

	if p.Seq == 0 {
		p.Seq = rp.Seq
	}

	if p.Count == 0 {
		p.Count = rp.Count
	}

	if p.Sent.Before(rp.Sent) {
		p.Sent = rp.Sent
	}

	if p.Recieved.Before(rp.Recieved) {
		p.Recieved = rp.Recieved
	}

	if p.TimeOut == 0 {
		p.TimeOut = rp.TimeOut
	}

	if p.Len == 0 {
		p.Len = rp.Len
	}

	if p.TTL == 0 {
		p.TTL = rp.TTL
	}
}

// RTT returns the RTT of the ping
func (p *Ping) RTT() time.Duration {
	if !p.Recieved.Before(p.Sent) {
		return p.Recieved.Sub(p.Sent)
	}
	return 0
}

// TimeOutTime returns the time this ping times out
func (p *Ping) TimeOutTime() time.Time {
	return p.Sent.Add(p.TimeOut)
}

func (p *Ping) sendType() icmp.Type {
	if p.Dst.IP.To4() != nil {
		return ipv4.ICMPTypeEcho
	}
	return ipv6.ICMPTypeEchoRequest
}

// ToICMPMsg returns a byte array ready to send on the wire
func (p *Ping) ToICMPMsg() ([]byte, error) {
	return (&icmp.Message{
		Code: 0,
		Type: p.sendType(),
		Body: &icmp.Echo{
			ID:   int(p.ID),
			Seq:  int(p.Seq),
			Data: TimeToBytes(p.Sent),
		},
	}).Marshal(nil)
}

// TimeToBytes converts a time.Time into a []byte for inclusion in the ICMP payload
func TimeToBytes(t time.Time) []byte {
	b := make([]byte, TimeSliceLength)
	binary.LittleEndian.PutUint64(b, uint64(t.UnixNano()))
	return b
}

// ErrTooShort is returned if an echo body holding the timeslice is too short
var ErrTooShort = errors.New("too short")

// BytesToTime converst a []byte into a time.Time
func BytesToTime(b []byte) (time.Time, error) {
	if len(b) < TimeSliceLength {
		return time.Time{}, ErrTooShort
	}
	return time.Unix(0, int64(binary.LittleEndian.Uint64(b[:TimeSliceLength]))), nil
}
