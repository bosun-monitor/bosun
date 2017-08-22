package collectors

import (
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: cVmstatDarwin})
}

func cVmstatDarwin() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var free float64
	util.ReadCommand(func(line string) error {
		if line == "" || strings.HasPrefix(line, "Object cache") || strings.HasPrefix(line, "Mach Virtual") {
			return nil
		}
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			return nil
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(fields[0], "Pages") {
			name := strings.TrimSpace(fields[0])
			name = strings.Replace(name, "Pages ", "", -1)
			name = strings.Replace(name, " ", "", -1)
			Add(&md, "darwin.mem.vm.4kpages."+name, value, nil, metadata.Unknown, metadata.None, "")
			if name == "free" {
				free = value * 4096
				Add(&md, osMemFree, free, nil, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
			}
		} else if fields[0] == "Pageins" {
			Add(&md, "darwin.mem.vm.pageins", value, nil, metadata.Counter, metadata.None, "")
		} else if fields[0] == "Pageouts" {
			Add(&md, "darwin.mem.vm.pageouts", value, nil, metadata.Counter, metadata.None, "")
		}
		return nil
	}, "vm_stat")
	util.ReadCommand(func(line string) error {
		total, _ := strconv.ParseFloat(line, 64)
		if total == 0 {
			return nil
		}
		Add(&md, osMemTotal, total, nil, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
		if free == 0 {
			return nil
		}
		Add(&md, osMemUsed, total-free, nil, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
		Add(&md, osMemPctFree, free/total, nil, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
		return nil
	}, "sysctl", "-n", "hw.memsize")
	return md, nil
}
