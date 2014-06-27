package collectors

import (
	"math"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_cpu_windows})
	collectors = append(collectors, &IntervalCollector{F: c_cpu_info_windows})
}

var cpuWindowsPrev uint64 = math.MaxUint64

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_PerfOS_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		slog.Infoln("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	var used, num uint64
	for _, v := range dst {
		used += v.Timestamp_Sys100NS - v.PercentIdleTime
		num++
		Add(&md, "win.cpu", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu", v.PercentIdleTime, opentsdb.TagSet{"cpu": v.Name, "type": "idle"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.interrupts", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.dpcs", v.InterruptsPersec, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.time_cstate", v.PercentC1Time, opentsdb.TagSet{"cpu": v.Name, "type": "c1"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.time_cstate", v.PercentC2Time, opentsdb.TagSet{"cpu": v.Name, "type": "c2"}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.time_cstate", v.PercentC3Time, opentsdb.TagSet{"cpu": v.Name, "type": "c3"}, metadata.Unknown, metadata.None, "")
	}
	if num > 0 {
		cpu := used / 1e5 / num
		a, b := float64(cpu), float64(cpuWindowsPrev)
		a, b = math.Max(a, b), math.Min(a, b)
		if d := a - b; d >= 0 && d <= 100 {
			Add(&md, osCPU, cpu, nil, metadata.Unknown, metadata.None, "")
		}
		cpuWindowsPrev = cpu
	}
	return md
}

type Win32_PerfRawData_PerfOS_Processor struct {
	DPCRate               uint32
	InterruptsPersec      uint32
	Name                  string
	PercentC1Time         uint64
	Timestamp_Sys100NS    uint64
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
		slog.Infoln("cpu_info:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.cpu.clock", v.CurrentClockSpeed, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.clock_max", v.MaxClockSpeed, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.voltage", v.CurrentVoltage, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.cores_physical", v.NumberOfCores, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "win.cpu.cores_logical", v.NumberOfLogicalProcessors, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		if v.LoadPercentage != nil {
			Add(&md, "win.cpu.load", *v.LoadPercentage, opentsdb.TagSet{"cpu": v.Name}, metadata.Unknown, metadata.None, "")
		}
	}
	return md
}

type Win32_Processor struct {
	CurrentClockSpeed         uint32
	CurrentVoltage            uint16
	LoadPercentage            *uint16
	MaxClockSpeed             uint32
	Name                      string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
}
