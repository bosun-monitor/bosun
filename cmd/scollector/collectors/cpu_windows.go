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
		Add(&md, "cpu.time_idle", v.PercentIdleTime, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "cpu.interrupts", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "cpu.dpcs", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "cpu.time_cstate", v.PercentC1Time, opentsdb.TagSet{"cpu": v.Name, "type": "c1"})
		Add(&md, "cpu.time_cstate", v.PercentC2Time, opentsdb.TagSet{"cpu": v.Name, "type": "c2"})
		Add(&md, "cpu.time_cstate", v.PercentC3Time, opentsdb.TagSet{"cpu": v.Name, "type": "c3"})
	}
	return md
}

type Win32_PerfRawData_PerfOS_Processor struct {
	DPCRate               uint32
	InterruptsPersec      uint32
	Name                  string
	PercentC1Time         uint64
	PercentC2Time         uint64
	PercentC3Time         uint64
	PercentIdleTime       uint64
	PercentInterruptTime  uint64
	PercentPrivilegedTime uint64
	PercentProcessorTime  uint64
	PercentUserTime       uint64
}
