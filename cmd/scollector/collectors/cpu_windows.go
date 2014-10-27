package collectors

import (
	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_cpu_windows})
	collectors = append(collectors, &IntervalCollector{F: c_cpu_info_windows})
}

func c_cpu_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfOS_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	var used, num uint64
	for _, v := range dst {
		ts := TSys100NStoEpoch(v.Timestamp_Sys100NS)
		num++
		//Divide by 1e5 because: 1 seconds / 100 Nanoseconds = 1e7. This is the percent time as a decimal, so divide by two less zeros to make it the same as the result * 100.
		used += (v.PercentUserTime + v.PercentPrivilegedTime + v.PercentInterruptTime) / 1e5
		AddTS(&md, "win.cpu", ts, v.PercentPrivilegedTime/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"}, metadata.Counter, metadata.Pct, "Percentage of non-idle processor time spent in privileged mode.")
		AddTS(&md, "win.cpu", ts, v.PercentInterruptTime/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"}, metadata.Counter, metadata.Pct, "Percentage of time that the processor spent receiving and servicing hardware interrupts during the sample interval.")
		AddTS(&md, "win.cpu", ts, v.PercentUserTime/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "user"}, metadata.Counter, metadata.Pct, "Percentage of non-idle processor time spent in user mode.")
		AddTS(&md, "win.cpu", ts, v.PercentIdleTime/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "idle"}, metadata.Counter, metadata.Pct, "Percentage of time during the sample interval that the processor was idle.")
		AddTS(&md, "win.cpu.interrupts", ts, v.InterruptsPersec/1e5, opentsdb.TagSet{"cpu": v.Name}, metadata.Counter, metadata.Event, "Average number of hardware interrupts that the processor is receiving and servicing in each second.")
		Add(&md, "win.cpu.dpcs", v.DPCRate, opentsdb.TagSet{"cpu": v.Name}, metadata.Counter, metadata.Event, "Rate at which deferred procedure calls (DPCs) are added to the processor DPC queue between the timer tics of the processor clock.")
		AddTS(&md, "win.cpu.time_cstate", ts, v.PercentC1Time/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "c1"}, metadata.Counter, metadata.Pct, "Percentage of time that the processor spends in the C1 low-power idle state, which is a subset of the total processor idle time.")
		AddTS(&md, "win.cpu.time_cstate", ts, v.PercentC2Time/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "c2"}, metadata.Counter, metadata.Pct, "Percentage of time that the processor spends in the C-2 low-power idle state, which is a subset of the total processor idle time.")
		AddTS(&md, "win.cpu.time_cstate", ts, v.PercentC3Time/1e5, opentsdb.TagSet{"cpu": v.Name, "type": "c3"}, metadata.Counter, metadata.Pct, "Percentage of time that the processor spends in the C3 low-power idle state, which is a subset of the total processor idle time.")
	}
	if num > 0 {
		cpu := used / num
		Add(&md, osCPU, cpu, nil, metadata.Counter, metadata.Pct, "")
	}
	return md, nil
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

func c_cpu_info_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_Processor
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.cpu.clock", v.CurrentClockSpeed, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.MHz, "Current speed of the processor, in MHz.")
		Add(&md, "win.cpu.clock_max", v.MaxClockSpeed, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.MHz, "Maximum speed of the processor, in MHz.")
		Add(&md, "win.cpu.voltage", v.CurrentVoltage, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.V_10, "Voltage of the processor.")
		Add(&md, "win.cpu.cores_physical", v.NumberOfCores, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.Count, "Number of cores for the current instance of the processor.")
		Add(&md, "win.cpu.cores_logical", v.NumberOfLogicalProcessors, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.Count, "Number of logical processors for the current instance of the processor.")
		if v.LoadPercentage != nil {
			Add(&md, "win.cpu.load", *v.LoadPercentage, opentsdb.TagSet{"cpu": v.Name}, metadata.Gauge, metadata.Pct, "Load capacity of each processor, averaged to the last second.")
		}
	}
	return md, nil
}

type Win32_Processor struct {
	CurrentClockSpeed         uint32
	CurrentVoltage            *uint16
	LoadPercentage            *uint16
	MaxClockSpeed             uint32
	Name                      string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
}
