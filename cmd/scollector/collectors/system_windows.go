package collectors

import (
	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_system_windows})
}

func c_system_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfOS_System
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		var uptime = (v.Timestamp_Object - v.SystemUpTime) / v.Frequency_Object //see http://microsoft.public.win32.programmer.wmi.narkive.com/09kqthVC/lastbootuptime
		Add(&md, "win.system.uptime", uptime, nil, metadata.Gauge, metadata.None, "Seconds since last reboot.")
		Add(&md, "win.system.processes", v.Processes, nil, metadata.Gauge, metadata.None, "Total running process count.")
		Add(&md, osSystemUptime, uptime, nil, metadata.Gauge, metadata.None, "Seconds since last reboot.")
	}
	return md, nil
}

type Win32_PerfRawData_PerfOS_System struct {
	Frequency_Object uint64
	Processes        uint32
	SystemUpTime     uint64
	Timestamp_Object uint64
}
