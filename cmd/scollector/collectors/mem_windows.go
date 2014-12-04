package collectors

import (
	"bosun.org/_third_party/github.com/StackExchange/wmi"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_simple_mem_windows})
}

// Memory needs to be expanded upon. Should be deeper in utilization (what is
// cache, etc.) as well as saturation (i.e., paging activity). Lot of that is in
// Win32_PerfRawData_PerfOS_Memory. Win32_Operating_System's units are KBytes.

func c_simple_mem_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_OperatingSystem
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.mem.vm.total", v.TotalVirtualMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, "Number, in bytes, of virtual memory.")
		Add(&md, "win.mem.vm.free", v.FreeVirtualMemory*1024, nil, metadata.Gauge, metadata.Bytes, "Number, in bytes, of virtual memory currently unused and available.")
		Add(&md, "win.mem.total", v.TotalVisibleMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, "Total amount, in bytes, of physical memory available to the operating system.")
		Add(&md, "win.mem.free", v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, "Number, in bytes, of physical memory currently unused and available.")
		Add(&md, osMemTotal, v.TotalVisibleMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, "Total amount, in bytes, of physical memory available to the operating system.")
		Add(&md, osMemFree, v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
		Add(&md, osMemUsed, v.TotalVisibleMemorySize*1024-v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, "")
		Add(&md, osMemPctFree, float64(v.FreePhysicalMemory)/float64(v.TotalVisibleMemorySize)*100, nil, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
	}
	return md, nil
}

type Win32_OperatingSystem struct {
	FreePhysicalMemory     uint64
	FreeVirtualMemory      uint64
	TotalVirtualMemorySize uint64
	TotalVisibleMemorySize uint64
}
