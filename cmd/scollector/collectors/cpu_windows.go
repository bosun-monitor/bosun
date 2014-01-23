package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_cpu_windows})
	collectors = append(collectors, &IntervalCollector{F: c_cpu_info_windows})
}

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_PerfOS_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.cpu", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"})
		Add(&md, "win.cpu", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"})
		Add(&md, "win.cpu", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"})
		Add(&md, "win.cpu", v.PercentIdleTime, opentsdb.TagSet{"cpu": v.Name, "type": "idle"})
		Add(&md, "win.cpu.interrupts", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.dpcs", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.time_cstate", v.PercentC1Time, opentsdb.TagSet{"cpu": v.Name, "type": "c1"})
		Add(&md, "win.cpu.time_cstate", v.PercentC2Time, opentsdb.TagSet{"cpu": v.Name, "type": "c2"})
		Add(&md, "win.cpu.time_cstate", v.PercentC3Time, opentsdb.TagSet{"cpu": v.Name, "type": "c3"})
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

func c_cpu_info_windows() opentsdb.MultiDataPoint {
	var dst []Win32_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		l.Println("cpu_info:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.cpu.clock", v.CurrentClockSpeed, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.clock_max", v.MaxClockSpeed, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.voltage", v.CurrentVoltage, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.load", v.LoadPercentage, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.cores_physical", v.NumberOfCores, opentsdb.TagSet{"cpu": v.Name})
		Add(&md, "win.cpu.cores_logical", v.NumberOfLogicalProcessors, opentsdb.TagSet{"cpu": v.Name})
	}
	return md
}

//This is actually a CIM_Processor according to C# reflection
type Win32_Processor struct {
	CurrentClockSpeed         uint32
	CurrentVoltage            uint16
	LoadPercentage            uint16
	MaxClockSpeed             uint32
	Name                      string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
}
