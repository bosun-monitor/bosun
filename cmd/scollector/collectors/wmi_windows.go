package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
	"regexp"
)

func init() {
	collectors = append(collectors, c_cpu_windows)
	collectors = append(collectors, c_network_windows)
	collectors = append(collectors, c_physical_disk_windows)
	collectors = append(collectors, c_simple_mem_windows)
}

const CPU_QUERY = `
	SELECT Name, PercentPrivilegedTime, PercentInterruptTime, PercentUserTime
	FROM Win32_PerfRawData_PerfOS_Processor
	WHERE Name <> '_Total'
`

//KMB Moving Excludes to Go, we didn't notice the WHERE changing performance much and a regex
//is easier than building the WHERE string
const NETWORK_QUERY = `
	SELECT Name, BytesReceivedPerSec, BytesSentPerSec,
		PacketsReceivedPerSec, PacketsSentPerSec,
		PacketsOutboundDiscarded, PacketsOutboundErrors,
		PacketsReceivedDiscarded, PacketsReceivedErrors
	FROM Win32_PerfRawData_Tcpip_NetworkInterface
`
const INTERFACE_EXCLUSIONS = `isatap|Teredo`

const PHYSICAL_DISK_QUERY = `
	SELECT Name, AvgDisksecPerRead, AvgDisksecPerWrite,
		AvgDiskReadQueueLength, AvgDiskWriteQueueLength, 
		DiskReadBytesPersec, DiskReadsPersec,
		DiskWriteBytesPersec, DiskWritesPersec, 
		SplitIOPerSec, PercentDiskReadTime, PercentDiskWriteTime
	FROM Win32_PerfRawData_PerfDisk_PhysicalDisk
`

//Memory Needs to be expanded upon, Should be deeper in Utilization (What is Cache etc) 
//as well as Saturation (i.e. Paging Activity). Lot of that is in Win32_PerfRawData_PerfOS_Memory

//Win32_Operating_System's units are KBytes
const SIMPLE_MEMORY_QUERY =  `
	SELECT FreePhysicalMemory, FreeVirtualMemory,
	TotalVisibleMemorySize, TotalVirtualMemorySize
	FROM Win32_OperatingSystem
`

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_PerfOS_Processor
	err := wmi.Query(CPU_QUERY, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "cpu.time", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"})
		Add(&md, "cpu.time", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"})
		Add(&md, "cpu.time", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"})
	}
	return md
}

func c_simple_mem_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_OperatingSystem
	err := wmi.Query(SIMPLE_MEMORY_QUERY, &dst)
	if err != nil {
		l.Println("simple_mem:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mem.virtual.total", v.TotalVirtualMemorySize * 1024, opentsdb.TagSet{})
		Add(&md, "mem.virtual.free", v.FreeVirtualMemory * 1024, opentsdb.TagSet{})
		Add(&md, "mem.physical.total", v.TotalVisibleMemorySize * 1024, opentsdb.TagSet{})
		Add(&md, "mem.physical.free", v.FreePhysicalMemory * 1024, opentsdb.TagSet{})
	}
	return md
}

func c_physical_disk_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_PerfDisk_PhysicalDisk
	err := wmi.Query(PHYSICAL_DISK_QUERY, &dst)
	if err != nil {
		l.Println("disk_physical:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "disk.physical.duration", v.AvgDiskSecPerRead, opentsdb.TagSet{"disk": v.Name, "type": "read"})
		Add(&md, "disk.physical.duration", v.AvgDiskSecPerWrite, opentsdb.TagSet{"disk": v.Name, "type": "write"})
		Add(&md, "disk.physical.queue", v.AvgDiskReadQueueLength, opentsdb.TagSet{"disk": v.Name, "type": "read"})
		Add(&md, "disk.physical.queue", v.AvgDiskWriteQueueLength, opentsdb.TagSet{"disk": v.Name, "type": "write"})
		Add(&md, "disk.physical.ops", v.DiskReadsPerSec, opentsdb.TagSet{"disk": v.Name, "type": "read"})
		Add(&md, "disk.physical.ops", v.DiskWritesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "write"})
		Add(&md, "disk.physical.bytes", v.DiskReadBytesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "read"})
		Add(&md, "disk.physical.bytes", v.DiskWriteBytesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "write"})
		Add(&md, "disk.physical.percenttime", v.PercentDiskReadTime, opentsdb.TagSet{"disk": v.Name, "type": "read"})
		Add(&md, "disk.physical.percenttime", v.PercentDiskWriteTime, opentsdb.TagSet{"disk": v.Name, "type": "write"})
		Add(&md, "disk.physical.spltio", v.SplitIOPerSec, opentsdb.TagSet{"disk": v.Name})
	}
	return md
}

func c_network_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_Tcpip_NetworkInterface
	err := wmi.Query(NETWORK_QUERY, &dst)
	if err != nil {
		l.Println("network:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	exclusions := regexp.MustCompile(INTERFACE_EXCLUSIONS)
	for _, v := range dst {
		if ! exclusions.MatchString(v.Name) {
			Add(&md, "network.bytes", v.BytesReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"})
			Add(&md, "network.bytes", v.BytesSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"})
			Add(&md, "network.packets", v.PacketsReceivedPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "in"})
			Add(&md, "network.packets", v.PacketsSentPerSec, opentsdb.TagSet{"iface": v.Name, "direction": "out"})
			Add(&md, "network.err", v.PacketsOutboundDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "out"})
			Add(&md, "network.err", v.PacketsReceivedDiscarded, opentsdb.TagSet{"iface": v.Name, "type": "discard", "direction": "in"})
			Add(&md, "network.err", v.PacketsOutboundErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "out"})
			Add(&md, "network.err", v.PacketsReceivedErrors, opentsdb.TagSet{"iface": v.Name, "type": "error", "direction": "in"})
		}
	}
	return md
}

