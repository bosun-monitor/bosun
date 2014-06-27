package collectors

import (
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_conntrack_linux})
}

func c_conntrack_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	var max, count float64
	if err := readLine("/proc/sys/net/netfilter/nf_conntrack_count", func(s string) {
		values := strings.Fields(s)
		if len(values) > 0 {
			var err error
			count, err = strconv.ParseFloat(values[0], 64)
			if err != nil {
				return
			}
			Add(&md, "linux.net.conntrack.count", count, nil, metadata.Unknown, metadata.None, "")
		}
	}); err != nil {
		return nil
	}
	if err := readLine("/proc/sys/net/netfilter/nf_conntrack_max", func(s string) {
		values := strings.Fields(s)
		if len(values) > 0 {
			var err error
			max, err = strconv.ParseFloat(values[0], 64)
			if err != nil {
				return
			}
			Add(&md, "linux.net.conntrack.max", max, nil, metadata.Unknown, metadata.None, "")
		}
	}); err != nil {
		return nil
	}
	if max != 0 {
		Add(&md, "linux.net.conntrack.percent_used", count/max*100, nil, metadata.Unknown, metadata.None, "")
	}
	return md
}
