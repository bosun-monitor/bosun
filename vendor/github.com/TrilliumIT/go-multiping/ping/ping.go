// Package ping provides an api for efficiently pinging lots of hosts using ICMP echos.
package ping

import (
	"net"
	"time"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

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
	// ID is the ICMP ID
	ID int
	// Seq is the ICMP Sequence
	Seq int
	// Count is the count of this ICMP
	Count int
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

func iPingToPing(p *ping.Ping) *Ping {
	if p == nil {
		return nil
	}
	rp := &Ping{
		Host:     p.Host,
		ID:       int(p.ID),
		Seq:      int(p.Seq),
		Count:    p.Count,
		Sent:     p.Sent,
		Recieved: p.Recieved,
		TimeOut:  p.TimeOut,
		TTL:      p.TTL,
		Len:      p.Len,
	}
	if p.Src != nil {
		rp.Src = &net.IPAddr{}
		*rp.Src = *p.Src
	}
	if p.Dst != nil {
		rp.Dst = &net.IPAddr{}
		*rp.Dst = *p.Dst
	}
	return rp
}
