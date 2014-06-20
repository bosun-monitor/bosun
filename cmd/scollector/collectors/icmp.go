package collectors

import (
	"fmt"
	"net"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
	"github.com/tatsushid/go-fastping"
)

type response struct {
	addr *net.IPAddr
	rtt  time.Duration
}

// ICMP registers an ICMP collector a given host.
func ICMP(host string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() opentsdb.MultiDataPoint {
			return c_icmp(host)
		},
		name: fmt.Sprintf("icmp-%s", host),
	})
}

func c_icmp(host string) opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	p := fastping.NewPinger()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		slog.Error(err)
		return nil
	}
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	timeout := 1
	p.AddHandler("receive", func(addr *net.IPAddr, t time.Duration) {
		Add(&md, "ping.rtt", float64(t)/float64(time.Millisecond), opentsdb.TagSet{"dst_host": host})
		timeout = 0
	})
	if err := p.Run(); err != nil {
		slog.Error(err)
		return nil
	}
	Add(&md, "ping.timeout", timeout, opentsdb.TagSet{"dst_host": host})
	return md
}
