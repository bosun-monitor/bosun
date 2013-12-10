package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, Collector{F: c_cpu_windows})
}

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_PerfOS_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := wmi.Query(q, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "cpu.time", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"})
		Add(&md, "cpu.time", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"})
		Add(&md, "cpu.time", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"})
	}
	return md
}

type Win32_PerfRawData_PerfOS_Processor struct {
	Name                  string
	PercentInterruptTime  uint64
	PercentPrivilegedTime uint64
	PercentUserTime       uint64
}
