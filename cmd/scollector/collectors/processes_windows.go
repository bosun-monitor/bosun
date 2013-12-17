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

func c_windows_processes() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_PerfProc_Process
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := wmi.Query(q, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		if !processInclusions.MatchString(v.Name) {
			continue
		}
		id := "0"
		raw_name := strings.Split(v.Name, "#")
		name := raw_name[0]
		if len(raw_name) == 2 {
			id = raw_name[1]
		}
		// If you have a hash sign in your process name you don't deserve monitoring ;-)
		if len(raw_name) > 2 {
			continue
		}
		Add(&md, "processes.elapsed_time", v.ElapsedTime, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.handle_count", v.HandleCount, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.io_bytes", v.IOOtherBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"})
		Add(&md, "processes.io_operations", v.IOOtherOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "other"})
		Add(&md, "processes.io_bytes", v.IOReadBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"})
		Add(&md, "processes.io_operations", v.IOReadOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "read"})
		Add(&md, "processes.io_bytes", v.IOWriteBytesPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"})
		Add(&md, "processes.io_operations", v.IOWriteOperationsPersec, opentsdb.TagSet{"name": name, "id": id, "type": "write"})
		Add(&md, "processes.page_faults", v.PageFaultsPersec, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.pagefile_bytes", v.PageFileBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.pagefile_bytes_peak", v.PageFileBytesPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.cpu_time", v.PercentPrivilegedTime, opentsdb.TagSet{"name": name, "id": id, "type": "privileged"})
		Add(&md, "processes.cpu_total", v.PercentProcessorTime, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.cpu_time", v.PercentUserTime, opentsdb.TagSet{"name": name, "id": id, "type": "user"})
		Add(&md, "processes.pool_nonpaged_bytes", v.PoolNonpagedBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.pool_paged_bytes", v.PoolPagedBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.priority_base", v.PriorityBase, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.private_bytes", v.PrivateBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.thread_count", v.ThreadCount, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.virtual_bytes", v.VirtualBytes, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.virtual_bytespeak", v.VirtualBytesPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.workingset", v.WorkingSet, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.workingset_peak", v.WorkingSetPeak, opentsdb.TagSet{"name": name, "id": id})
		Add(&md, "processes.workingset_private", v.WorkingSetPrivate, opentsdb.TagSet{"name": name, "id": id})
	}
	return md
}

// Actually a CIM_StatisticalInformation Struct according to Reflection
type Win32_PerfRawData_PerfProc_Process struct {
	ElapsedTime             uint64
	HandleCount             uint32
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
