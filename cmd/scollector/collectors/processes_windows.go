package collectors

import (
	"fmt"
	"regexp"
	"strings"

	"bosun.org/_third_party/github.com/StackExchange/wmi"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var regexesProcesses = []*regexp.Regexp{}

func AddProcessConfig(params conf.ProcessParams) error {
	if params.Name == "" {
		return fmt.Errorf("empty process Name")
	}
	reg, err := regexp.Compile(params.Name)
	if err != nil {
		return err
	}
	regexesProcesses = append(regexesProcesses, reg)
	return nil
}

func WatchProcesses() {
	if len(regexesProcesses) == 0 {
		// if no process settings configured in config file, use this set instead.
		regexesProcesses = append(regexesProcesses, regexp.MustCompile("chrome|powershell|scollector|WinRM|MSSQLSERVER"))
	}
	collectors = append(collectors, &IntervalCollector{
		F: c_windows_processes,
	})
}

func c_windows_processes() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfProc_Process
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}

	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, "")
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		return nil, err
	}

	var iis_dst []WorkerProcess
	iis_q := wmi.CreateQuery(&iis_dst, "")
	err = queryWmiNamespace(iis_q, &iis_dst, "root\\WebAdministration")
	if err != nil {
		// Don't return from this error since the name space might exist.
		iis_dst = nil
	}

	var numberOfLogicalProcessors uint64
	var core_dst []Win32_ComputerSystem
	var core_q = wmi.CreateQuery(&core_dst, "")
	err = queryWmi(core_q, &core_dst)
	if err != nil {
		return nil, err
	}
	for _, y := range core_dst {
		numberOfLogicalProcessors = uint64(y.NumberOfLogicalProcessors)
	}
	if numberOfLogicalProcessors == 0 {
		return nil, fmt.Errorf("invalid result: numberOfLogicalProcessors=%v", numberOfLogicalProcessors)
	}

	var md opentsdb.MultiDataPoint
	var svc_dst_started []Win32_Service
	for _, svc := range svc_dst {
		if nameMatches(svc.Name, regexesProcesses) {
			if svc.Started {
				svc_dst_started = append(svc_dst_started, svc)
			}
			tags := opentsdb.TagSet{"name": svc.Name}
			Add(&md, "win.service.started", btoi(svc.Started), tags, metadata.Gauge, metadata.Bool, descWinServiceStarted)
			Add(&md, "win.service.status", btoi(svc.Status != "OK"), tags, metadata.Gauge, metadata.Ok, descWinServiceStatus)
			Add(&md, "win.service.checkpoint", svc.CheckPoint, tags, metadata.Gauge, metadata.None, descWinServiceCheckPoint)
			Add(&md, "win.service.wait_hint", svc.WaitHint, tags, metadata.Gauge, metadata.MilliSecond, descWinServiceWaitHint)
		}
	}

	for _, v := range dst {
		var name string
		service_match := false
		iis_match := false

		process_match := nameMatches(v.Name, regexesProcesses)

		id := "0"

		if process_match {
			raw_name := strings.Split(v.Name, "#")
			name = raw_name[0]
			if len(raw_name) == 2 {
				id = raw_name[1]
			}
			// If you have a hash sign in your process name you don't deserve monitoring ;-)
			if len(raw_name) > 2 {
				continue
			}
		}

		// A Service match could "overwrite" a process match, but that is probably what we would want
		for _, svc := range svc_dst_started {
			// It is possible the pid has gone and been reused, but I think this unlikely
			// And I'm not aware of an atomic join we could do anyways
			if svc.ProcessId != 0 && svc.ProcessId == v.IDProcess {
				id = "0"
				service_match = true
				name = svc.Name
				break
			}
		}

		for _, a_pool := range iis_dst {
			if a_pool.ProcessId == v.IDProcess {
				id = "0"
				iis_match = true
				name = strings.Join([]string{"iis", a_pool.AppPoolName}, "_")
				break
			}
		}

		if !(service_match || process_match || iis_match) {
			continue
		}

		//Use timestamp from WMI to fix issues with CPU metrics
		ts := TSys100NStoEpoch(v.Timestamp_Sys100NS)
		tags := opentsdb.TagSet{"name": name, "id": id}
		AddTS(&md, "win.proc.cpu", ts, v.PercentPrivilegedTime/NS100_Seconds/numberOfLogicalProcessors, opentsdb.TagSet{"type": "privileged"}.Merge(tags), metadata.Counter, metadata.Pct, descWinProcCPU_priv)
		AddTS(&md, "win.proc.cpu", ts, v.PercentUserTime/NS100_Seconds/numberOfLogicalProcessors, opentsdb.TagSet{"type": "user"}.Merge(tags), metadata.Counter, metadata.Pct, descWinProcCPU_user)
		AddTS(&md, "win.proc.cpu_total", ts, v.PercentProcessorTime/NS100_Seconds/numberOfLogicalProcessors, tags, metadata.Counter, metadata.Pct, descWinProcCPU_total)
		Add(&md, "win.proc.elapsed_time", (v.Timestamp_Object-v.ElapsedTime)/v.Frequency_Object, tags, metadata.Gauge, metadata.Second, descWinProcElapsed_time)
		Add(&md, "win.proc.handle_count", v.HandleCount, tags, metadata.Gauge, metadata.Count, descWinProcHandle_count)
		Add(&md, "win.proc.io_bytes", v.IOOtherBytesPersec, opentsdb.TagSet{"type": "other"}.Merge(tags), metadata.Counter, metadata.BytesPerSecond, descWinProcIo_bytes_other)
		Add(&md, "win.proc.io_bytes", v.IOReadBytesPersec, opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.BytesPerSecond, descWinProcIo_bytes_read)
		Add(&md, "win.proc.io_bytes", v.IOWriteBytesPersec, opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.BytesPerSecond, descWinProcIo_bytes_write)
		Add(&md, "win.proc.io_operations", v.IOOtherOperationsPersec, opentsdb.TagSet{"type": "other"}.Merge(tags), metadata.Counter, metadata.Operation, descWinProcIo_operations)
		Add(&md, "win.proc.io_operations", v.IOReadOperationsPersec, opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Operation, descWinProcIo_operations_read)
		Add(&md, "win.proc.io_operations", v.IOWriteOperationsPersec, opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Operation, descWinProcIo_operations_write)
		Add(&md, "win.proc.mem.page_faults", v.PageFaultsPersec, tags, metadata.Counter, metadata.PerSecond, descWinProcMemPage_faults)
		Add(&md, "win.proc.mem.pagefile_bytes", v.PageFileBytes, tags, metadata.Gauge, metadata.Bytes, descWinProcMemPagefile_bytes)
		Add(&md, "win.proc.mem.pagefile_bytes_peak", v.PageFileBytesPeak, tags, metadata.Gauge, metadata.Bytes, descWinProcMemPagefile_bytes_peak)
		Add(&md, "win.proc.mem.pool_nonpaged_bytes", v.PoolNonpagedBytes, tags, metadata.Gauge, metadata.Bytes, descWinProcMemPool_nonpaged_bytes)
		Add(&md, "win.proc.mem.pool_paged_bytes", v.PoolPagedBytes, tags, metadata.Gauge, metadata.Bytes, descWinProcMemPool_paged_bytes)
		Add(&md, "win.proc.mem.vm.bytes", v.VirtualBytes, tags, metadata.Gauge, metadata.Bytes, descWinProcMemVmBytes)
		Add(&md, "win.proc.mem.vm.bytes_peak", v.VirtualBytesPeak, tags, metadata.Gauge, metadata.Bytes, descWinProcMemVmBytes_peak)
		Add(&md, "win.proc.mem.working_set", v.WorkingSet, tags, metadata.Gauge, metadata.Bytes, descWinProcMemWorking_set)
		Add(&md, "win.proc.mem.working_set_peak", v.WorkingSetPeak, tags, metadata.Gauge, metadata.Bytes, descWinProcMemWorking_set_peak)
		Add(&md, "win.proc.mem.working_set_private", v.WorkingSetPrivate, tags, metadata.Gauge, metadata.Bytes, descWinProcMemWorking_set_private)
		Add(&md, "win.proc.priority_base", v.PriorityBase, tags, metadata.Gauge, metadata.None, descWinProcPriority_base)
		Add(&md, "win.proc.private_bytes", v.PrivateBytes, tags, metadata.Gauge, metadata.Bytes, descWinProcPrivate_bytes)
		Add(&md, "win.proc.thread_count", v.ThreadCount, tags, metadata.Gauge, metadata.Count, descWinProcthread_count)
	}
	return md, nil
}

func nameMatches(name string, regexes []*regexp.Regexp) bool {
	for _, r := range regexes {
		if r.MatchString(name) {
			return true
		}
	}
	return false
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Divide CPU by 1e5 because: 1 seconds / 100 Nanoseconds = 1e7. This is the
// percent time as a decimal, so divide by two less zeros to make it the same as
// the result * 100.
const NS100_Seconds = 1e5

const (
	descWinProcCPU_priv               = "Percentage of elapsed time that this thread has spent executing code in privileged mode."
	descWinProcCPU_total              = "Percentage of elapsed time that this process's threads have spent executing code in user or privileged mode."
	descWinProcCPU_user               = "Percentage of elapsed time that this process's threads have spent executing code in user mode."
	descWinProcElapsed_time           = "Elapsed time in seconds this process has been running."
	descWinProcHandle_count           = "Total number of handles the process has open across all threads."
	descWinProcIo_bytes_other         = "Rate at which the process is issuing bytes to I/O operations that do not involve data such as control operations."
	descWinProcIo_bytes_read          = "Rate at which the process is reading bytes from I/O operations."
	descWinProcIo_bytes_write         = "Rate at which the process is writing bytes to I/O operations."
	descWinProcIo_operations          = "Rate at which the process is issuing I/O operations that are neither a read or a write request."
	descWinProcIo_operations_read     = "Rate at which the process is issuing read I/O operations."
	descWinProcIo_operations_write    = "Rate at which the process is issuing write I/O operations."
	descWinProcMemPage_faults         = "Rate of page faults by the threads executing in this process."
	descWinProcMemPagefile_bytes      = "Current number of bytes this process has used in the paging file(s)."
	descWinProcMemPagefile_bytes_peak = "Maximum number of bytes this process has used in the paging file(s)."
	descWinProcMemPool_nonpaged_bytes = "Total number of bytes for objects that cannot be written to disk when they are not being used."
	descWinProcMemPool_paged_bytes    = "Total number of bytes for objects that can be written to disk when they are not being used."
	descWinProcMemVmBytes             = "Current size, in bytes, of the virtual address space that the process is using."
	descWinProcMemVmBytes_peak        = "Maximum number of bytes of virtual address space that the process has used at any one time."
	descWinProcMemWorking_set         = "Current number of bytes in the working set of this process at any point in time."
	descWinProcMemWorking_set_peak    = "Maximum number of bytes in the working set of this process at any point in time."
	descWinProcMemWorking_set_private = "Current number of bytes in the working set that are not shared with other processes."
	descWinProcPriority_base          = "Current base priority of this process. Threads within a process can raise and lower their own base priority relative to the process base priority of the process."
	descWinProcPrivate_bytes          = "Current number of bytes this process has allocated that cannot be shared with other processes."
	descWinProcthread_count           = "Number of threads currently active in this process."
)

// Actually a CIM_StatisticalInformation.
type Win32_PerfRawData_PerfProc_Process struct {
	ElapsedTime             uint64
	Frequency_Object        uint64
	HandleCount             uint32
	IDProcess               uint32
	IOOtherBytesPersec      uint64
	IOOtherOperationsPersec uint64
	IOReadBytesPersec       uint64
	IOReadOperationsPersec  uint64
	IOWriteBytesPersec      uint64
	IOWriteOperationsPersec uint64
	Name                    string
	PageFaultsPersec        uint32
	PageFileBytes           uint64
	PageFileBytesPeak       uint64
	PercentPrivilegedTime   uint64
	PercentProcessorTime    uint64
	PercentUserTime         uint64
	PoolNonpagedBytes       uint32
	PoolPagedBytes          uint32
	PriorityBase            uint32
	PrivateBytes            uint64
	ThreadCount             uint32
	Timestamp_Object        uint64
	Timestamp_Sys100NS      uint64
	VirtualBytes            uint64
	VirtualBytesPeak        uint64
	WorkingSet              uint64
	WorkingSetPeak          uint64
	WorkingSetPrivate       uint64
}

const (
	descWinServiceCheckPoint = "The CheckPoint property specifies a value that the service increments periodically to report its progress during a lengthy start, stop, pause, or continue operation. For example, the service should increment this value as it completes each step of its initialization when it is starting up. The user interface program that invoked the operation on the service uses this value to track the progress of the service during a lengthy operation. This value is not valid and should be zero when the service does not have a start, stop, pause, or continue operation pending."
	descWinServiceStarted    = "Started is a boolean indicating whether the service has been started (TRUE), or stopped (FALSE)."
	descWinServiceStatus     = "The Status property indicates the current status of the object. Right now 0=OK and 1=Not OK, but various operational and non-operational statuses can be defined such as OK, Degraded,  Pred Fail, Error, Starting, Stopping, and Service."
	descWinServiceWaitHint   = "The WaitHint property specifies the estimated time required (in milliseconds) for a pending start, stop, pause, or continue operation. After the specified amount of time has elapsed, the service makes its next call to the SetServiceStatus function with either an incremented CheckPoint value or a change in Current State. If the amount of time specified by WaitHint passes, and CheckPoint has not been incremented, or the Current State has not changed, the service control manager or service control program assumes that an error has occurred."
)

// Actually a Win32_BaseServce.
type Win32_Service struct {
	CheckPoint uint32
	Name       string
	ProcessId  uint32
	Started    bool
	Status     string
	WaitHint   uint32
}

type WorkerProcess struct {
	AppPoolName string
	ProcessId   uint32
}
