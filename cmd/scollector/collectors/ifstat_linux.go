package collectors

import (
	"regexp"
	"strings"

	"github.com/bosun-monitor/scollector/metadata"
	"github.com/bosun-monitor/scollector/opentsdb"
	"github.com/bosun-monitor/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_ifstat_linux})
	collectors = append(collectors, &IntervalCollector{F: c_ipcount_linux})
}

var netFields = []struct {
	key  string
	rate metadata.RateType
	unit metadata.Unit
}{
	{"bytes", metadata.Counter, metadata.Bytes},
	{"packets", metadata.Counter, metadata.Count},
	{"errs", metadata.Counter, metadata.Count},
	{"dropped", metadata.Counter, metadata.Count},
	{"fifo.errs", metadata.Counter, metadata.Count},
	{"frame.errs", metadata.Counter, metadata.Count},
	{"compressed", metadata.Counter, metadata.Count},
	{"multicast", metadata.Counter, metadata.Count},
	{"bytes", metadata.Counter, metadata.Bytes},
	{"packets", metadata.Counter, metadata.Count},
	{"errs", metadata.Counter, metadata.Count},
	{"dropped", metadata.Counter, metadata.Count},
	{"fifo.errs", metadata.Counter, metadata.Count},
	{"collisions", metadata.Counter, metadata.Count},
	{"carrier.errs", metadata.Counter, metadata.Count},
	{"compressed", metadata.Counter, metadata.Count},
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
				Add(&md, "linux.net.bond."+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				}, netFields[i].rate, netFields[i].unit, "")

				if i < 4 || (i >= 8 && i < 12) {
					Add(&md, "os.net.bond."+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
						"iface":     intf,
						"direction": direction(i),
					}, netFields[i].rate, netFields[i].unit, "")

				}
			} else {
				Add(&md, "linux.net."+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				}, netFields[i].rate, netFields[i].unit, "")

				if i < 4 || (i >= 8 && i < 12) {
					Add(&md, "os.net."+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
						"iface":     intf,
						"direction": direction(i),
					}, netFields[i].rate, netFields[i].unit, "")

				}
			}
		}
		return nil
	})
	return md, err
}
