package collectors

import (
	"regexp"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_windows_processes})
}

// These are silly processes but exist on my machine, will need to update KMB
var processInclusions = regexp.MustCompile("chrome|powershell|scollector|SocketServer")
var serviceInclusions = regexp.MustCompile("WinRM")

func c_windows_processes() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfProc_Process
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}

	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, `WHERE Name <> '_Total'`)
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		return nil, err
	}

	var iis_dst []WorkerProcess
	iis_q := wmi.CreateQuery(&iis_dst, "")
	err = queryWmiNamespace(iis_q, &iis_dst, "root\\WebAdministration")
	if err != nil {
		//Don't Return from this error since the name space might exist
		iis_dst = nil
	}

	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		var name string
		service_match := false
		iis_match := false
		process_match := processInclusions.MatchString(v.Name)

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
		for _, svc := range svc_dst {
			if serviceInclusions.MatchString(svc.Name) {
				// It is possible the pid has gone and been reused, but I think this unlikely
				// And I'm not aware of an atomic join we could do anyways
				if svc.ProcessId == v.IDProcess {
					id = "0"
					service_match = true
					name = svc.Name
					break
				}
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

		Add(&md, "win.proc.elapsed_time", (v.Timestamp_Object-v.ElapsedTime)/v.Frequency_Object, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Second, "Elapsed time in seconds this process has been running.")
		Add(&md, "win.proc.handle_count", v.HandleCount, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Count, "Total number of handles the process has open across all threads.")
		Add(&md, "win.proc.io_bytes", v.IOOtherBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"}, metadata.Counter, metadata.BytesPerSecond, "Rate at which the process is issuing bytes to I/O operations that don not involve data such as control operations.")
		Add(&md, "win.proc.io_operations", v.IOOtherOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"}, metadata.Counter, metadata.Operation, "Rate at which the process is issuing I/O operations that are neither a read or a write request.")
		Add(&md, "win.proc.io_bytes", v.IOReadBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"}, metadata.Counter, metadata.BytesPerSecond, "Rate at which the process is reading bytes from I/O operations.")
		Add(&md, "win.proc.io_operations", v.IOReadOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"}, metadata.Counter, metadata.Operation, "Rate at which the process is issuing read I/O operations.")
		Add(&md, "win.proc.io_bytes", v.IOWriteBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"}, metadata.Counter, metadata.BytesPerSecond, "Rate at which the process is writing bytes to I/O operations.")
		Add(&md, "win.proc.io_operations", v.IOWriteOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"}, metadata.Counter, metadata.Operation, "Rate at which the process is issuing write I/O operations.")
		Add(&md, "win.proc.mem.page_faults", v.PageFaultsPersec, opentsdb.TagSet{"name": name, "id": id}, metadata.Counter, metadata.PerSecond, "Rate of page faults by the threads executing in this process.")
		Add(&md, "win.proc.mem.pagefile_bytes", v.PageFileBytes, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Current number of bytes this process has used in the paging file(s).")
		Add(&md, "win.proc.mem.pagefile_bytes_peak", v.PageFileBytesPeak, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Maximum number of bytes this process has used in the paging file(s).")
		//Divide CPU by 1e5 because: 1 seconds / 100 Nanoseconds = 1e7. This is the percent time as a decimal, so divide by two less zeros to make it the same as the result * 100.
		Add(&md, "win.proc.cpu", v.PercentPrivilegedTime/1e5, opentsdb.TagSet{"name": name, "id": id, "type": "privileged"}, metadata.Counter, metadata.Pct, "Percentage of elapsed time that this thread has spent executing code in privileged mode.")
		Add(&md, "win.proc.cpu_total", v.PercentProcessorTime/1e5, opentsdb.TagSet{"name": name, "id": id}, metadata.Counter, metadata.Pct, "Percentage of elapsed time that this process's threads have spent executing code in user or privileged mode.")
		Add(&md, "win.proc.cpu", v.PercentUserTime/1e5, opentsdb.TagSet{"name": name, "id": id, "type": "user"}, metadata.Counter, metadata.Pct, "Percentage of elapsed time that this process's threads have spent executing code in user mode.")
		Add(&md, "win.proc.mem.pool_nonpaged_bytes", v.PoolNonpagedBytes, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Total number of bytes for objects that cannot be written to disk when they are not being used.")
		Add(&md, "win.proc.mem.pool_paged_bytes", v.PoolPagedBytes, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Total number of bytes for objects that can be written to disk when they are not being used.")
		Add(&md, "win.proc.priority_base", v.PriorityBase, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.None, "Current base priority of this process. Threads within a process can raise and lower their own base priority relative to the process base priority of the process.")
		Add(&md, "win.proc.private_bytes", v.PrivateBytes, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Current number of bytes this process has allocated that cannot be shared with other processes.")
		Add(&md, "win.proc.thread_count", v.ThreadCount, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Count, "Number of threads currently active in this process.")
		Add(&md, "win.proc.mem.vm.bytes", v.VirtualBytes, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Current size, in bytes, of the virtual address space that the process is using.")
		Add(&md, "win.proc.mem.vm.bytes_peak", v.VirtualBytesPeak, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Maximum number of bytes of virtual address space that the process has used at any one time.")
		Add(&md, "win.proc.mem.working_set", v.WorkingSet, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Current number of bytes in the working set of this process at any point in time.")
		Add(&md, "win.proc.mem.working_set_peak", v.WorkingSetPeak, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Maximum number of bytes in the working set of this process at any point in time.")
		Add(&md, "win.proc.mem.working_set_private", v.WorkingSetPrivate, opentsdb.TagSet{"name": name, "id": id}, metadata.Gauge, metadata.Bytes, "Current number of bytes in the working set that are not shared with other processes.")

	}
	return md, nil
}

// Actually a CIM_StatisticalInformation Struct according to Reflection
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
	VirtualBytes            uint64
	VirtualBytesPeak        uint64
	WorkingSet              uint64
	WorkingSetPeak          uint64
	WorkingSetPrivate       uint64
}

//Actually a Win32_BaseServce
type Win32_Service struct {
	Name      string
	ProcessId uint32
}

type WorkerProcess struct {
	AppPoolName string
	ProcessId   uint32
}
