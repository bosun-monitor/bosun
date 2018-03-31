package collectors

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"

	"github.com/TrilliumIT/go-multiping/ping"
)

type ICMPCollector struct {
	host     string
	interval time.Duration
	timeout  time.Duration
	TagOverride
}

func (i *ICMPCollector) Name() string {
	return fmt.Sprintf("icmp-%s", i.host)
}

func (c *ICMPCollector) Init() {
	rand.Seed(time.Now().UnixNano())
}

func (i *ICMPCollector) Run(dpchan chan<- *opentsdb.DataPoint, quit <-chan struct{}) {
	ctx, cancel := context.WithCancel(context.Background())

	handle := func(p *ping.Ping, err error) {
		var md opentsdb.MultiDataPoint

		_, resolveFailed := err.(*net.DNSError)
		AddTS(&md, "ping.resolved", p.Sent.Unix(), !resolveFailed, opentsdb.TagSet{"dst_host": i.host}, metadata.Unknown, metadata.None, "")

		timedOut := err != nil
		AddTS(&md, "ping.timeout", p.Sent.Unix(), timedOut, opentsdb.TagSet{"dst_host": i.host}, metadata.Unknown, metadata.None, "")

		if p.RTT() != 0 {
			AddTS(&md, "ping.rtt", p.Sent.Unix(), float64(p.RTT())/float64(time.Millisecond), opentsdb.TagSet{"dst_host": i.host}, metadata.Unknown, metadata.None, "")
		}

		if p.TTL != 0 {
			AddTS(&md, "ping.ttl", p.Sent.Unix(), p.TTL, opentsdb.TagSet{"dst_host": i.host}, metadata.Unknown, metadata.None, "")
		}

		for _, dp := range md {
			i.ApplyTagOverrides(dp.Tags)
			select {
			case <-ctx.Done():
				return
			case dpchan <- dp:
			}
		}
	}

	time.AfterFunc(
		time.Duration(rand.Int63n(int64(i.interval))),
		func() { ping.HostInterval(ctx, i.host, 1, handle, 0, i.interval, i.timeout) },
	)

	<-quit
	cancel()
}

// ICMP registers an ICMP collector a given host.
func ICMP(host, interval, timeout string) error {
	if host == "" {
		return fmt.Errorf("empty ICMP hostname")
	}
	ping.SetWorkers(2)

	i := DefaultFreq
	t := time.Second
	var err error

	if interval != "" {
		i, err = time.ParseDuration(interval)
		if err != nil {
			i := DefaultFreq
			slog.Errorf("Error parsing interval: %v, using default of %v.", err, i)
		}
	}

	if timeout != "" {
		t, err = time.ParseDuration(timeout)
		if err != nil {
			t = time.Second
			slog.Errorf("Error parsing timeout: %v, using default of %v.", err, t)
		}
	}

	collectors = append(collectors, &ICMPCollector{
		host:     host,
		interval: i,
		timeout:  t,
	})
	return nil
}
