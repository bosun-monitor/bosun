package collectors

import (
	"regexp"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, c_procstats_linux)
}

var uptimeRE = regexp.MustCompile(`(\S+)\s+(\S+)`)
var meminfoRE = regexp.MustCompile(`(\w+):\s+(\d+)\s+(\w+)`)
var vmstatRE = regexp.MustCompile(`(\w+)\s+(\d+)`)

func c_procstats_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readProc("/proc/uptime", func(s string) {
		m := uptimeRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "proc.uptime.total", m[1], nil)
		Add(&md, "proc.uptime.now", m[2], nil)
	})
	readProc("/proc/meminfo", func(s string) {
		m := meminfoRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "proc.meminfo."+strings.ToLower(m[1]), m[2], nil)
	})
	readProc("/proc/vmstat", func(s string) {
		m := vmstatRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		switch m[1] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout", "pgfault", "pgmajfault":
			Add(&md, "proc.vmstat."+m[1], m[2], nil)
		}
	})
	return md
}
