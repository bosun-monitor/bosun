// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package conn

import (
	"net"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func setupV4Conn(c *ipv4.PacketConn) error {
	err := c.SetControlMessage(ipv4.FlagDst|ipv4.FlagSrc|ipv4.FlagTTL, true)
	if err != nil {
		return err
	}

	var f ipv4.ICMPFilter
	f.SetAll(true)
	f.Accept(ipv4.ICMPTypeEchoReply)
	err = c.SetICMPFilter(&f)
	return err
}

func setupV6Conn(c *ipv6.PacketConn) error {
	err := c.SetControlMessage(ipv6.FlagDst|ipv6.FlagSrc|ipv6.FlagHopLimit, true)
	if err != nil {
		return err
	}
	var f ipv6.ICMPFilter
	f.SetAll(true)
	f.Accept(ipv6.ICMPTypeEchoReply)
	err = c.SetICMPFilter(&f)
	return err
}

func readV4(c *ipv4.PacketConn, len int) (
	payload []byte,
	srcAddr net.Addr,
	src, dst net.IP,
	rlen, ttl int,
	received time.Time,
	err error,
) {
	payload = make([]byte, len)
	var cm *ipv4.ControlMessage
	rlen, cm, srcAddr, err = c.ReadFrom(payload)
	received = time.Now()
	if cm != nil {
		src, dst, ttl = cm.Src, cm.Dst, cm.TTL
	}
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
	payload = make([]byte, len)
	var cm *ipv6.ControlMessage
	rlen, cm, srcAddr, err = c.ReadFrom(payload)
	received = time.Now()
	if cm != nil {
		src, dst, ttl = cm.Src, cm.Dst, cm.HopLimit
	}
	return
}
