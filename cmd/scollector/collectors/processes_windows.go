package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
	"regexp"
	"strings"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_windows_processes})
}

// These are silly processes but exist on my machine, will need to update KMB
var processInclusions = regexp.MustCompile("chrome|powershell|tcollector")
var serviceInclusions = regexp.MustCompile("WinRM")

func c_windows_processes() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_PerfProc_Process
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		l.Println("processes:", err)
		return nil
	}

	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, `WHERE Name <> '_Total'`)
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		l.Println("services:", err)
		return nil
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

		Add(&md, "win.proc.elapsed_time", v.ElapsedTime, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.handle_count", v.HandleCount, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.io_bytes", v.IOOtherBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"})
		Add(&md, "win.proc.io_operations", v.IOOtherOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"})
		Add(&md, "win.proc.io_bytes", v.IOReadBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"})
		Add(&md, "win.proc.io_operations", v.IOReadOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"})
		Add(&md, "win.proc.io_bytes", v.IOWriteBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"})
		Add(&md, "win.proc.io_operations", v.IOWriteOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"})
		Add(&md, "win.proc.mem.page_faults", v.PageFaultsPersec, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.pagefile_bytes", v.PageFileBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.pagefile_bytes_peak", v.PageFileBytesPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.cpu", v.PercentPrivilegedTime, opentsdb.TagSet{"name": name, "id": id, "type": "privileged"})
		Add(&md, "win.proc.cpu_total", v.PercentProcessorTime, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.cpu", v.PercentUserTime, opentsdb.TagSet{"name": name, "id": id, "type": "user"})
		Add(&md, "win.proc.mem.pool_nonpaged_bytes", v.PoolNonpagedBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.pool_paged_bytes", v.PoolPagedBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.priority_base", v.PriorityBase, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.private_bytes", v.PrivateBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.thread_count", v.ThreadCount, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.vm.bytes", v.VirtualBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.vm.bytes_peak", v.VirtualBytesPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.working_set", v.WorkingSet, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.working_set_peak", v.WorkingSetPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "win.proc.mem.working_set_private", v.WorkingSetPrivate, opentsdb.TagSet{"name": name, "id": id})

	}
	return md
}

// Actually a CIM_StatisticalInformation Struct according to Reflection
type Win32_PerfRawData_PerfProc_Process struct {
	ElapsedTime             uint64
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
