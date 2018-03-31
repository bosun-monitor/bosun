// see https://github.com/golang/net/blob/master/ipv4/control_windows.go#L14
package conn

import (
	"net"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func setupV4Conn(c *ipv4.PacketConn) error {
	return nil
}

func setupV6Conn(c *ipv6.PacketConn) error {
	return nil
}

func readV4(c *ipv4.PacketConn, len int) (
	payload []byte,
	srcAddr net.Addr,
	src, dst net.IP,
	rlen, ttl int,
	received time.Time,
	err error,
) {
	payload = make([]byte, len+ipv4.HeaderLen)
	rlen, _, srcAddr, err = c.ReadFrom(payload)
	received = time.Now()
	src, dst = net.IPv4zero, net.IPv4zero
	return
}

func readV6(c *ipv6.PacketConn, len int) (
	payload []byte,
	srcAddr net.Addr,
	src, dst net.IP,
	rlen, ttl int,
	received time.Time,
	err error,
) {
	payload = make([]byte, len+ipv6.HeaderLen)
	rlen, _, srcAddr, err = c.ReadFrom(payload)
	received = time.Now()
	src, dst = net.IPv6zero, net.IPv6zero
	return
}
