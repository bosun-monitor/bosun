package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_physical_disk_windows)
	collectors = append(collectors, c_diskspace_windows)
}

const PHYSICAL_DISK_QUERY = `
	SELECT Name, AvgDisksecPerRead, AvgDisksecPerWrite,
		AvgDiskReadQueueLength, AvgDiskWriteQueueLength, 
		DiskReadBytesPersec, DiskReadsPersec,
		DiskWriteBytesPersec, DiskWritesPersec, 
		SplitIOPerSec, PercentDiskReadTime, PercentDiskWriteTime
	FROM Win32_PerfRawData_PerfDisk_PhysicalDisk
	WHERE Name <> '_Total'
`

//Similar Breakdowns exist as to physical, but for now just using this for the space utilization
const DISKSPACE_QUERY = `
	SELECT Name, FreeMegaBytes, PercentFreeSpace
	FROM Win32_PerfRawData_PerfDisk_LogicalDisk
	WHERE Name <> '_Total'
`

func c_diskspace_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_PerfDisk_LogicalDisk
	err := wmi.Query(DISKSPACE_QUERY, &dst)
	if err != nil {
		l.Println("diskpace:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "disk.logical.free_bytes", v.FreeMegabytes*1048576, opentsdb.TagSet{"partition": v.Name})
		Add(&md, "disk.logical.percent_free", v.PercentFreeSpace, opentsdb.TagSet{"partition": v.Name})
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
