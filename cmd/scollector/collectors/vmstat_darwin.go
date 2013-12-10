package collectors

import (
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, Collector{c_vmstat_darwin, DEFAULT_FREQ_SEC})
}

func c_vmstat_darwin() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readCommand(func(line string) {
		if line == "" || strings.HasPrefix(line, "Object cache") || strings.HasPrefix(line, "Mach Virtual") {
			return
		}
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			return
		}
		value := strings.TrimSpace(fields[1])
		value = strings.Replace(value, ".", "", -1)
		if strings.HasPrefix(fields[0], "Pages") {
			name := strings.TrimSpace(fields[0])
			name = strings.Replace(name, "Pages ", "", -1)
			name = strings.Replace(name, " ", "", -1)
			Add(&md, "vm.4kpages."+name, value, nil)
		} else if fields[0] == "Pageins" {
			Add(&md, "vm.pageins", value, nil)
		} else if fields[0] == "Pageouts" {
			Add(&md, "vm.pageouts", value, nil)
		}
	}, "vm_stat")
	return md
}
