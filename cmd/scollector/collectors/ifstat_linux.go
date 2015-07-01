package collectors

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
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

var teamRegexp = regexp.MustCompile(`^team\d+`)

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
		// Skip headers
		if strings.Contains(s, "|") {
			return nil
		}
		m := strings.Fields(s)
		intf := strings.TrimRight(m[0], ":")
		stats := m[1:]
		tags := opentsdb.TagSet{"iface": intf}

		// Detect non-ethernet device types
		var namespace_string string
		_ = readLine("/sys/class/net/"+intf+"/type", func(devType string) error {
			if devType != "1" {
				namespace_string = "virtual."
			}
			return nil
		})
		// Detect virtual ethernet devices types
		if namespace_string == "" {
			if _, err := os.Stat("/sys/class/net/" + intf + "/bonding"); !os.IsNotExist(err) {
				// Bond interface
				namespace_string = "bond."
			} else if teamRegexp.MatchString(intf) {
				// Team interface matched via regex (unreliable)
				namespace_string = "bond."
			} else {
				// Generic virtual device detection
				devPath, err := filepath.EvalSymlinks("/sys/class/net/" + intf)
				if err != nil {
					return nil
				}
				if strings.Contains(devPath, "/virtual/") {
					namespace_string = "virtual."
				}
			}
		}

		// Detect speed of the interface in question
		_ = readLine("/sys/class/net/"+intf+"/speed", func(speed string) error {
			Add(&md, "linux.net."+namespace_string+"ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, "")
			Add(&md, "os.net."+namespace_string+"ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, "")
			return nil
		})
		for i, v := range stats {
			Add(&md, "linux.net."+namespace_string+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
				"iface":     intf,
				"direction": direction(i),
			}, netFields[i].rate, netFields[i].unit, "")
			if i < 4 || (i >= 8 && i < 12) {
				Add(&md, "os.net."+namespace_string+strings.Replace(netFields[i].key, ".", "_", -1), v, opentsdb.TagSet{
					"iface":     intf,
					"direction": direction(i),
				}, netFields[i].rate, netFields[i].unit, "")

			}
		}
		return nil
	})
	return md, err
}
