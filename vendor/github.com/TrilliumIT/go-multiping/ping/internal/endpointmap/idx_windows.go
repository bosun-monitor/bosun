package endpointmap

import (
	"net"

	"github.com/TrilliumIT/go-multiping/ping/internal/ping"
)

func toIP4Idx(ip net.IP, id ping.ID) [6]byte {
	return iToIP4Idx(net.IPv4zero, id)
}

func toIP6Idx(ip net.IP, id ping.ID) [18]byte {
	return iToIP6Idx(net.IPv6zero, id)
}
