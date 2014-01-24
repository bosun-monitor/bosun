package collectors

import (
	"regexp"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_ifstat_linux})
}

var FIELDS_NET = []string{
	"bytes",
	"packets",
	"errs",
	"dropped",
	"fifo.errs",
	"frame.errs",
	"compressed",
	"multicast",
	"bytes",
	"packets",
	"errs",
	"dropped",
	"fifo.errs",
	"collisions",
	"carrier.errs",
	"compressed",
}

var ifstatRE = regexp.MustCompile(`\s+(eth\d+|em\d+_\d+/\d+|em\d+_\d+|em\d+|` +
	`bond\d+|` + `p\d+p\d+_\d+/\d+|p\d+p\d+_\d+|p\d+p\d+):(.*)`)

func c_ifstat_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	direction := func(i int) string {
		if i >= 8 {
			return "out"
		} else {
			return "in"
		}
	}
	readLine("/proc/net/dev", func(s string) {
		m := ifstatRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		intf := m[1]
		stats := strings.Fields(m[2])
		for i, v := range stats {
			if strings.HasPrefix(intf, "bond") {
				Add(&md, "linux.net.bond."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				})
			} else {
				Add(&md, "linux.net."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				})
			}
		}
	})
	return md
}
