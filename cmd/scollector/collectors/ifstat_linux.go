package collectors

import (
	"regexp"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_ifstat_linux})
	collectors = append(collectors, &IntervalCollector{F: c_ipcount_linux})
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

func c_ipcount_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	v4c := 0
	v6c := 0
	err := util.ReadCommand(func(line string) error {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "inet ") {
			v4c++
		}
		if strings.HasPrefix(tl, "inet6 ") {
			v6c++
		}
		return nil
	}, "ip", "addr", "list")
	if err != nil {
		return md, err
	}
	Add(&md, "linux.net.ip_count", v4c, opentsdb.TagSet{"version": "4"}, metadata.Gauge, "IP_Addresses", "")
	Add(&md, "linux.net.ip_count", v6c, opentsdb.TagSet{"version": "6"}, metadata.Gauge, "IP_Addresses", "")
	return md, nil
}

func c_ifstat_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	direction := func(i int) string {
		if i >= 8 {
			return "out"
		} else {
			return "in"
		}
	}
	err := readLine("/proc/net/dev", func(s string) error {
		m := ifstatRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		intf := m[1]
		stats := strings.Fields(m[2])
		tags := opentsdb.TagSet{"iface": intf}

		// Detect speed of the interface in question
		readLine("/sys/class/net/"+intf+"/speed", func(speed string) error {
			Add(&md, "linux.net.ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, "")
			Add(&md, "os.net.ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, "")
			return nil
		})
		for i, v := range stats {
			if strings.HasPrefix(intf, "bond") {
				Add(&md, "linux.net.bond."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				}, metadata.Unknown, metadata.None, "")  //TODO: different units

				if i < 4 || (i >= 8 && i < 12) {
					Add(&md, "os.net.bond."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
						"iface":     intf,
						"direction": direction(i),
					}, metadata.Unknown, metadata.None, "")  //TODO: different units

				}
			} else {
				Add(&md, "linux.net."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				}, metadata.Unknown, metadata.None, "")  //TODO: different units

				if i < 4 || (i >= 8 && i < 12) {
					Add(&md, "os.net."+strings.Replace(FIELDS_NET[i], ".", "_", -1), v, opentsdb.TagSet{
						"iface":     intf,
						"direction": direction(i),
					}, metadata.Unknown, metadata.None, "")  //TODO: different units

				}
			}
		}
		return nil
	})
	return md, err
}
