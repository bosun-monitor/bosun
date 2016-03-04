package collectors

import (
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_cpu_windows})
	collectors = append(collectors, &IntervalCollector{F: c_cpu_info_windows})
}

func c_cpu_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfOS_Processor
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	var used, num uint64
	var winCPUTotalPerfOS *Win32_PerfRawData_PerfOS_Processor
	for _, v := range dst {
		if v.Name == "_Total" {
			winCPUTotalPerfOS = &v
			continue
		}
		ts := TSys100NStoEpoch(v.Timestamp_Sys100NS)
		tags := opentsdb.TagSet{"cpu": v.Name}
		num++
		//Divide by 1e5 because: 1 seconds / 100 Nanoseconds = 1e7. This is the percent time as a decimal, so divide by two less zeros to make it the same as the result * 100.
		used += (v.PercentUserTime + v.PercentPrivilegedTime + v.PercentInterruptTime) / 1e5
		AddTS(&md, winCPU, ts, v.PercentPrivilegedTime/1e5, opentsdb.TagSet{"type": "privileged"}.Merge(tags), metadata.Counter, metadata.Pct, descWinCPUPrivileged)
		AddTS(&md, winCPU, ts, v.PercentInterruptTime/1e5, opentsdb.TagSet{"type": "interrupt"}.Merge(tags), metadata.Counter, metadata.Pct, descWinCPUInterrupt)
		AddTS(&md, winCPU, ts, v.PercentUserTime/1e5, opentsdb.TagSet{"type": "user"}.Merge(tags), metadata.Counter, metadata.Pct, descWinCPUUser)
		AddTS(&md, winCPU, ts, v.PercentIdleTime/1e5, opentsdb.TagSet{"type": "idle"}.Merge(tags), metadata.Counter, metadata.Pct, descWinCPUIdle)
		AddTS(&md, "win.cpu.interrupts", ts, v.InterruptsPersec, tags, metadata.Counter, metadata.Event, descWinCPUInterrupts)
		Add(&md, "win.cpu.dpcs", v.DPCRate, tags, metadata.Gauge, metadata.Event, descWinCPUDPC)
	}
	if num > 0 {
		cpu := used / num
		Add(&md, osCPU, cpu, nil, metadata.Counter, metadata.Pct, "")
	}
	if winCPUTotalPerfOS != nil {
		v := winCPUTotalPerfOS
		ts := TSys100NStoEpoch(v.Timestamp_Sys100NS)
		AddTS(&md, winCPUTotal, ts, v.PercentPrivilegedTime/1e5, opentsdb.TagSet{"type": "privileged"}, metadata.Counter, metadata.Pct, descWinCPUPrivileged)
		AddTS(&md, winCPUTotal, ts, v.PercentInterruptTime/1e5, opentsdb.TagSet{"type": "interrupt"}, metadata.Counter, metadata.Pct, descWinCPUInterrupt)
		AddTS(&md, winCPUTotal, ts, v.PercentUserTime/1e5, opentsdb.TagSet{"type": "user"}, metadata.Counter, metadata.Pct, descWinCPUUser)
		AddTS(&md, winCPUTotal, ts, v.PercentIdleTime/1e5, opentsdb.TagSet{"type": "idle"}, metadata.Counter, metadata.Pct, descWinCPUIdle)
		AddTS(&md, "win.cpu_total.interrupts", ts, v.InterruptsPersec, nil, metadata.Counter, metadata.Event, descWinCPUInterrupts)
		Add(&md, "win.cpu_total.dpcs", v.DPCRate, nil, metadata.Gauge, metadata.Event, descWinCPUDPC)
		AddTS(&md, winCPUCStates, ts, v.PercentC1Time/1e5, opentsdb.TagSet{"cpu": "total", "type": "c1"}, metadata.Counter, metadata.Pct, descWinCPUC1)
		AddTS(&md, winCPUCStates, ts, v.PercentC2Time/1e5, opentsdb.TagSet{"cpu": "total", "type": "c2"}, metadata.Counter, metadata.Pct, descWinCPUC2)
		AddTS(&md, winCPUCStates, ts, v.PercentC3Time/1e5, opentsdb.TagSet{"cpu": "total", "type": "c3"}, metadata.Counter, metadata.Pct, descWinCPUC3)
	}
	return md, nil
}

const (
	winCPU               = "win.cpu"
	winCPUTotal          = "win.cpu_total"
	winCPUCStates        = "win.cpu.time_cstate"
	descWinCPUPrivileged = "Percentage of non-idle processor time spent in privileged mode."
	descWinCPUInterrupt  = "Percentage of time that the processor spent receiving and servicing hardware interrupts during the sample interval."
	descWinCPUUser       = "Percentage of non-idle processor time spent in user mode."
	descWinCPUIdle       = "Percentage of time during the sample interval that the processor was idle."
	descWinCPUInterrupts = "Average number of hardware interrupts that the processor is receiving and servicing in each second."
	descWinCPUDPC        = "Rate at which deferred procedure calls (DPCs) are added to the processor DPC queue between the timer tics of the processor clock."
	descWinCPUC1         = "Percentage of time that the processor spends in the C1 low-power idle state, which is a subset of the total processor idle time."
	descWinCPUC2         = "Percentage of time that the processor spends in the C-2 low-power idle state, which is a subset of the total processor idle time."
	descWinCPUC3         = "Percentage of time that the processor spends in the C3 low-power idle state, which is a subset of the total processor idle time."
)

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
		tags := opentsdb.TagSet{"cpu": strings.Replace(v.DeviceID, "CPU", "", 1)}
		Add(&md, osCPUClock, v.CurrentClockSpeed, tags, metadata.Gauge, metadata.MHz, osCPUClockDesc)
		Add(&md, "win.cpu.clock", v.CurrentClockSpeed, tags, metadata.Gauge, metadata.MHz, descWinCPUClock)
		Add(&md, "win.cpu.clock_max", v.MaxClockSpeed, tags, metadata.Gauge, metadata.MHz, descWinCPUClockMax)
		Add(&md, "win.cpu.voltage", v.CurrentVoltage, tags, metadata.Gauge, metadata.V10, descWinCPUVoltage)
		Add(&md, "win.cpu.cores_physical", v.NumberOfCores, tags, metadata.Gauge, metadata.Count, descWinCPUCores)
		Add(&md, "win.cpu.cores_logical", v.NumberOfLogicalProcessors, tags, metadata.Gauge, metadata.Count, descWinCPUCoresLogical)
		if v.LoadPercentage != nil {
			Add(&md, "win.cpu.load", *v.LoadPercentage, tags, metadata.Gauge, metadata.Pct, descWinCPULoad)
		}
	}
	return md, nil
}

const (
	descWinCPUClock        = "Current speed of the processor, in MHz."
	descWinCPUClockMax     = "Maximum speed of the processor, in MHz."
	descWinCPUVoltage      = "Voltage of the processor."
	descWinCPUCores        = "Number of cores for the current instance of the processor."
	descWinCPUCoresLogical = "Number of logical processors for the current instance of the processor."
	descWinCPULoad         = "Load capacity of each processor, averaged to the last second."
)

type Win32_Processor struct {
	CurrentClockSpeed         uint32
	CurrentVoltage            *uint16
	LoadPercentage            *uint16
	MaxClockSpeed             uint32
	DeviceID                  string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
}
