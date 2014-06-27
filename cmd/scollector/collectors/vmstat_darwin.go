package collectors

import (
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_vmstat_darwin})
}

func c_vmstat_darwin() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) {
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
			Add(&md, "darwin.mem.vm.4kpages."+name, value, nil, metadata.Unknown, metadata.None, "")
		} else if fields[0] == "Pageins" {
			Add(&md, "darwin.mem.vm.pageins", value, nil, metadata.Counter, metadata.None, "")
		} else if fields[0] == "Pageouts" {
			Add(&md, "darwin.mem.vm.pageouts", value, nil, metadata.Counter, metadata.None, "")
		}
	}, "vm_stat")
	return md
}
