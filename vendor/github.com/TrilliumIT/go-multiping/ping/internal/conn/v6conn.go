package conn

import (
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

type v6Conn struct {
	icmpConn
}

func (c *v6Conn) start() error {
	var err error
	c.c, err = icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return err
	}
	err = setupV6Conn(c.c.IPv6PacketConn())
	return err
}

func (c *v6Conn) read() (*ping.Ping, error) {
	payload, srcAddr, src, dst, rlen, ttl, received, err := readV6(c.c.IPv6PacketConn(), 18)
	p := toPing(srcAddr, src, dst, rlen, ttl, received)
	if err != nil {
		return p, err
	}
	p.ID, p.Seq, p.Sent, err = parseEcho(ping.ProtocolIPv6ICMP,
		ipv6.ICMPTypeEchoReply, payload, rlen)
	return p, err
}
