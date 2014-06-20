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
		Interval: time.Second * 15,
		name:     fmt.Sprintf("icmp-%s", host),
	})
}

func c_icmp(host string) opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	p := fastping.NewPinger()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		slog.Error(err)
	}
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	p.AddHandler("receive", func(addr *net.IPAddr, t time.Duration) {
		Add(&md, "ping.rtt", float64(t)/float64(time.Millisecond), opentsdb.TagSet{"dst_host": host})
	})
	p.AddHandler("idle", func() {
		if len(md) < 1 {
			Add(&md, "ping.timeout", 1, opentsdb.TagSet{"dst_host": host})
		}
	})
	p.Run()
	return md
}
