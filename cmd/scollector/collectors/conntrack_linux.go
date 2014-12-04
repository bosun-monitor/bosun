package collectors

import (
	"os"
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_conntrack_linux, Enable: conntrackEnable})
}

const (
	conntrackCount = "/proc/sys/net/netfilter/nf_conntrack_count"
	conntrackMax   = "/proc/sys/net/netfilter/nf_conntrack_max"
)

func conntrackEnable() bool {
	f, err := os.Open(conntrackCount)
	defer f.Close()
	return err == nil
}

func c_conntrack_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var max, count float64
	if err := readLine(conntrackCount, func(s string) error {
		values := strings.Fields(s)
		if len(values) > 0 {
			var err error
			count, err = strconv.ParseFloat(values[0], 64)
			if err != nil {
				return nil
			}
			Add(&md, "linux.net.conntrack.count", count, nil, metadata.Gauge, metadata.Count, "")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := readLine(conntrackMax, func(s string) error {
		values := strings.Fields(s)
		if len(values) > 0 {
			var err error
			max, err = strconv.ParseFloat(values[0], 64)
			if err != nil {
				return nil
			}
			Add(&md, "linux.net.conntrack.max", max, nil, metadata.Gauge, metadata.Count, "")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if max != 0 {
		Add(&md, "linux.net.conntrack.percent_used", count/max*100, nil, metadata.Gauge, metadata.Pct, "")
	}
	return md, nil
}
