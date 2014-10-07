package collectors

import (
	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_physical_disk_windows})
	collectors = append(collectors, &IntervalCollector{F: c_diskspace_windows})
}

func c_diskspace_windows() (opentsdb.MultiDataPoint, error) {
	const megabyte = 1048576
	var dst []Win32_PerfFormattedData_PerfDisk_LogicalDisk
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.disk.fs.space_free", v.FreeMegabytes*megabyte, opentsdb.TagSet{"partition": v.Name}, metadata.Gauge, metadata.Bytes, "")
		Add(&md, osDiskFree, v.FreeMegabytes*megabyte, opentsdb.TagSet{"disk": v.Name}, metadata.Gauge, metadata.Bytes, "")
		if v.PercentFreeSpace != 0 {
			space_total := v.FreeMegabytes * megabyte * 100 / v.PercentFreeSpace
			space_used := space_total - v.FreeMegabytes*megabyte
			Add(&md, "win.disk.fs.space_total", space_total, opentsdb.TagSet{"partition": v.Name}, metadata.Gauge, metadata.Bytes, "")
			Add(&md, "win.disk.fs.space_used", space_used, opentsdb.TagSet{"partition": v.Name}, metadata.Gauge, metadata.Bytes, "")
			Add(&md, osDiskTotal, space_total, opentsdb.TagSet{"disk": v.Name}, metadata.Gauge, metadata.Bytes, "")
			Add(&md, osDiskUsed, space_used, opentsdb.TagSet{"disk": v.Name}, metadata.Gauge, metadata.Bytes, "")
		}

		Add(&md, "win.disk.fs.percent_free", v.PercentFreeSpace, opentsdb.TagSet{"partition": v.Name}, metadata.Gauge, metadata.Pct, "")
		Add(&md, osDiskPctFree, v.PercentFreeSpace, opentsdb.TagSet{"disk": v.Name}, metadata.Gauge, metadata.Pct, "")
	}
	return md, nil
}

type Win32_PerfFormattedData_PerfDisk_LogicalDisk struct {
	FreeMegabytes    uint64
	Name             string
	PercentFreeSpace uint64
}

func c_physical_disk_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfDisk_PhysicalDisk
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.disk.duration", v.AvgDiskSecPerRead/1000000, opentsdb.TagSet{"disk": v.Name, "type": "read"}, metadata.Counter, metadata.MilliSecond, "Time, in milliseconds, of a read from the disk")
		Add(&md, "win.disk.duration", v.AvgDiskSecPerWrite/1000000, opentsdb.TagSet{"disk": v.Name, "type": "write"}, metadata.Counter, metadata.MilliSecond, "Time, in milliseconds, of a write to the disk")
		Add(&md, "win.disk.queue", v.AvgDiskReadQueueLength/10000000, opentsdb.TagSet{"disk": v.Name, "type": "read"}, metadata.Counter, metadata.Operation, "Number of read requests that were queued for the disk")
		Add(&md, "win.disk.queue", v.AvgDiskWriteQueueLength/10000000, opentsdb.TagSet{"disk": v.Name, "type": "write"}, metadata.Counter, metadata.Operation, "Number of write requests that were queued for the disk")
		Add(&md, "win.disk.ops", v.DiskReadsPerSec, opentsdb.TagSet{"disk": v.Name, "type": "read"}, metadata.Counter, metadata.PerSecond, "Number of read operations on the disk")
		Add(&md, "win.disk.ops", v.DiskWritesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "write"}, metadata.Counter, metadata.PerSecond, "Number of write operations on the disk")
		Add(&md, "win.disk.bytes", v.DiskReadBytesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "read"}, metadata.Counter, metadata.BytesPerSecond, "Number of bytes read from the disk")
		Add(&md, "win.disk.bytes", v.DiskWriteBytesPerSec, opentsdb.TagSet{"disk": v.Name, "type": "write"}, metadata.Counter, metadata.BytesPerSecond, "Number of bytes written to the disk")
		Add(&md, "win.disk.percent_time", v.PercentDiskReadTime/100000, opentsdb.TagSet{"disk": v.Name, "type": "read"}, metadata.Counter, metadata.Pct, "Percentage of time that the disk was busy servicing read requests")
		Add(&md, "win.disk.percent_time", v.PercentDiskWriteTime/100000, opentsdb.TagSet{"disk": v.Name, "type": "write"}, metadata.Counter, metadata.Pct, "Percentage of time that the disk was busy servicing write requests")
		Add(&md, "win.disk.spltio", v.SplitIOPerSec, opentsdb.TagSet{"disk": v.Name}, metadata.Counter, metadata.PerSecond, "Number of requests to the disk that were split into multiple requests due to size or fragmentation")
	}
	return md, nil
}

//See msdn for counter types http://msdn.microsoft.com/en-us/library/ms804035.aspx
type Win32_PerfRawData_PerfDisk_PhysicalDisk struct {
	AvgDiskReadQueueLength  uint64
	AvgDiskSecPerRead       uint32
	AvgDiskSecPerWrite      uint32
	AvgDiskWriteQueueLength uint64
	DiskReadBytesPerSec     uint64
	DiskReadsPerSec         uint32
	DiskWriteBytesPerSec    uint64
	DiskWritesPerSec        uint32
	Name                    string
	PercentDiskReadTime     uint64
	PercentDiskWriteTime    uint64
	SplitIOPerSec           uint32
}
