package collectors

import (
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, c_vmstat_darwin)
}

func c_vmstat_darwin() opentsdb.MultiDataPoint {
	b, err := command("vm_stat")
	if err != nil {
		l.Println("vmstat", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	s := string(b)
	for _, line := range strings.Split(s, "\n") {
		if line == "" || strings.HasPrefix(line, "Object cache") || strings.HasPrefix(line, "Mach Virtual") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
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
	}
	return md
}
