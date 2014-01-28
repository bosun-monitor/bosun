package collectors

import (
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_simple_mem_windows})
}

// Memory Needs to be expanded upon. Should be deeper in utilization (what is
// cache, etc.) as well as saturation (i.e., paging activity). Lot of that is in
// Win32_PerfRawData_PerfOS_Memory. Win32_Operating_System's units are KBytes.

func c_simple_mem_windows() opentsdb.MultiDataPoint {
	var dst []Win32_OperatingSystem
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		l.Println("simple_mem:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.mem.vm.total", v.TotalVirtualMemorySize*1024, nil)
		Add(&md, "win.mem.vm.free", v.FreeVirtualMemory*1024, nil)
		Add(&md, "win.mem.total", v.TotalVisibleMemorySize*1024, nil)
		Add(&md, "win.mem.free", v.FreePhysicalMemory*1024, nil)
	}
	return md
}

type Win32_OperatingSystem struct {
	FreePhysicalMemory     uint64
	FreeVirtualMemory      uint64
	TotalVirtualMemorySize uint64
	TotalVisibleMemorySize uint64
}
