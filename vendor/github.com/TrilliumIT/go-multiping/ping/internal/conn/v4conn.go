package conn

import (
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

type v4Conn struct {
	icmpConn
}

func (c *v4Conn) start() error {
	var err error
	c.c, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return err
	}
	err = setupV4Conn(c.c.IPv4PacketConn())
	return err
}

func (c *v4Conn) read() (*ping.Ping, error) {
	payload, srcAddr, src, dst, rlen, ttl, received, err := readV4(c.c.IPv4PacketConn(), 18)
	p := toPing(srcAddr, src, dst, rlen, ttl, received)
	if err != nil {
		return p, err
	}
	p.ID, p.Seq, p.Sent, err = parseEcho(ping.ProtocolICMP,
		ipv4.ICMPTypeEchoReply, payload, rlen)
	return p, err
}
