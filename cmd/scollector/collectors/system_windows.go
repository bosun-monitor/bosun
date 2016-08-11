package collectors

import (
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_system_windows})
	collectors = append(collectors, &IntervalCollector{F: c_windows_process_total})
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
		if v.Frequency_Object != 0 {
			//see http://microsoft.public.win32.programmer.wmi.narkive.com/09kqthVC/lastbootuptime
			var uptime = (v.Timestamp_Object - v.SystemUpTime) / v.Frequency_Object
			Add(&md, "win.system.uptime", uptime, nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
			Add(&md, osSystemUptime, uptime, nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
		}
		Add(&md, "win.system.context_switches", v.ContextSwitchesPersec, nil, metadata.Counter, metadata.ContextSwitch, descWinSystemContextSwitchesPersec)
		Add(&md, "win.system.exceptions", v.ExceptionDispatchesPersec, nil, metadata.Counter, metadata.PerSecond, descWinSystemExceptionDispatchesPersec)
		Add(&md, "win.system.cpu_queue", v.ProcessorQueueLength, nil, metadata.Gauge, metadata.Count, descWinSystemProcessorQueueLength)
		Add(&md, "win.system.syscall", v.SystemCallsPersec, nil, metadata.Counter, metadata.Syscall, descWinSystemSystemCallsPersec)
		Add(&md, "win.system.threads", v.Threads, nil, metadata.Gauge, metadata.Count, descWinSystemThreads)
		Add(&md, "win.system.processes", v.Processes, nil, metadata.Gauge, metadata.Count, descWinSystemProcesses)

	}
	return md, nil
}

const (
	descWinSystemContextSwitchesPersec     = "Context Switches/sec is the combined rate at which all processors on the computer are switched from one thread to another.  Context switches occur when a running thread voluntarily relinquishes the processor, is preempted by a higher priority ready thread, or switches between user-mode and privileged (kernel) mode to use an Executive or subsystem service.  It is the sum of Thread\\Context Switches/sec for all threads running on all processors in the computer and is measured in numbers of switches."
	descWinSystemExceptionDispatchesPersec = "Exception Dispatches/sec is the rate, in incidents per second, at which exceptions were dispatched by the system."
	descWinSystemProcesses                 = "The number of processes running on the system."
	descWinSystemProcessorQueueLength      = "Processor Queue Length is the number of threads in the processor queue.  Unlike the disk counters, this counter shows ready threads only, not threads that are running.  There is a single queue for processor time even on computers with multiple processors. Therefore, if a computer has multiple processors, you need to divide this value by the number of processors servicing the workload. A sustained processor queue of less than 10 threads per processor is normally acceptable, dependent on the workload."
	descWinSystemSystemCallsPersec         = "System Calls/sec is the combined rate of calls to operating system service routines by all processes running on the computer. These routines perform all of the basic scheduling and synchronization of activities on the computer, and provide access to non-graphic devices, memory management, and name space management."
	descWinSystemThreads                   = "The number of threads running on the system."
)

type Win32_PerfRawData_PerfOS_System struct {
	ContextSwitchesPersec     uint32
	ExceptionDispatchesPersec uint32
	Frequency_Object          uint64
	Processes                 uint32
	ProcessorQueueLength      uint32
	SystemCallsPersec         uint32
	SystemUpTime              uint64
	Threads                   uint32
	Timestamp_Object          uint64
}

func c_windows_process_total() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfProc_Process
	var q = wmi.CreateQuery(&dst, `WHERE Name = '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.system.handle_count", v.HandleCount, nil, metadata.Gauge, metadata.Count, descWinSystemHandle_count)
	}
	return md, nil
}

const (
	descWinSystemHandle_count = "Total number of handles open across all threads."
)
