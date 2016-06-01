package collectors

import (
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	var dst []win32PerfRawDataEVAPMEXTHPEVAStorageArray
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return // No HP EVA datasources found
	}
	collectors = append(collectors, &IntervalCollector{F: cHPEvaVirtualDiskWindows})
	collectors = append(collectors, &IntervalCollector{F: cHPEvaHostConnectionWindows})
	collectors = append(collectors, &IntervalCollector{F: cHPEvaStorageControllerWindows})
	collectors = append(collectors, &IntervalCollector{F: cHPEvaStorageArrayWindows})
	collectors = append(collectors, &IntervalCollector{F: cHPEvaHostPortWindows})
	collectors = append(collectors, &IntervalCollector{F: cHPEvaPhysicalDiskGroupWindows})
}

const (
	//Converts 1000nS samples to 1mS samples
	hpEvaDisk1000nS1mS float64 = 1000000.0
)

func cHPEvaVirtualDiskWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAVirtualDisk
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.vdisk.ops", v.ReadHitReqPers, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "hit"}, metadata.Counter, metadata.PerSecond, descVirtualDiskReqPers)
		Add(&md, "hp.eva.vdisk.ops", v.ReadMissReqPers, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "miss"}, metadata.Counter, metadata.PerSecond, descVirtualDiskReqPers)
		Add(&md, "hp.eva.vdisk.ops", v.WriteReqPers, opentsdb.TagSet{"vdisk": v.Name, "type": "write"}, metadata.Counter, metadata.PerSecond, descVirtualDiskReqPers)

		Add(&md, "hp.eva.vdisk.latency", float64(v.ReadHitLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "hit"}, metadata.Gauge, metadata.MilliSecond, descVirtualDiskLatencyus)
		Add(&md, "hp.eva.vdisk.latency", float64(v.ReadMissLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "miss"}, metadata.Gauge, metadata.MilliSecond, descVirtualDiskLatencyus)
		Add(&md, "hp.eva.vdisk.latency", float64(v.WriteLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"vdisk": v.Name, "type": "write"}, metadata.Gauge, metadata.MilliSecond, descVirtualDiskLatencyus)

		Add(&md, "hp.eva.vdisk.bytes", v.ReadHitKBPers*1024, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "hit"}, metadata.Counter, metadata.BytesPerSecond, descVirtualDiskKBPers)
		Add(&md, "hp.eva.vdisk.bytes", v.ReadMissKBPers*1024, opentsdb.TagSet{"vdisk": v.Name, "type": "read", "subtype": "miss"}, metadata.Counter, metadata.BytesPerSecond, descVirtualDiskKBPers)
		Add(&md, "hp.eva.vdisk.bytes", v.WriteKBPers*1024, opentsdb.TagSet{"vdisk": v.Name, "type": "write"}, metadata.Counter, metadata.BytesPerSecond, descVirtualDiskKBPers)
	}
	return md, nil
}

const (
	descVirtualDiskReqPers   = "HP EVA Virtual Disk performance data: Requests per second."
	descVirtualDiskLatencyus = "HP EVA Virtual Disk performance data: Latency time in milliseconds."
	descVirtualDiskKBPers    = "HP EVA Virtual Disk performance data: Throughput in Bytes per second."
)

//See msdn for counter types http://msdn.microsoft.com/en-us/library/ms804035.aspx
type win32PerfRawDataEVAPMEXTHPEVAVirtualDisk struct {
	Name              string
	ReadHitKBPers     uint64
	ReadHitLatencyus  uint64
	ReadHitReqPers    uint32
	ReadMissKBPers    uint64
	ReadMissLatencyus uint64
	ReadMissReqPers   uint32
	WriteKBPers       uint64
	WriteLatencyus    uint64
	WriteReqPers      uint64
}

func cHPEvaHostConnectionWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAHostConnection
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.hostconnection.queuedepth", v.QueueDepth, opentsdb.TagSet{"evahost": v.Name}, metadata.Gauge, metadata.Count, descHostConnectionQueueDepth)
	}
	return md, nil
}

const (
	descHostConnectionQueueDepth = "HP EVA host connections: Connection queue depth."
)

type win32PerfRawDataEVAPMEXTHPEVAHostConnection struct {
	Name       string
	QueueDepth uint16 // HP EVA host connections: Connection queue depth
}

func cHPEvaStorageControllerWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAStorageController
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.storagecontroller.transfer", v.PercentDataTransferTime, opentsdb.TagSet{"controller": v.Name}, metadata.Gauge, metadata.Pct, descStoragePercentDataTransferTime)
		Add(&md, "hp.eva.storagecontroller.cpu", v.PercentProcessorTime, opentsdb.TagSet{"controller": v.Name}, metadata.Gauge, metadata.Pct, descStoragePercentProcessorTime)
	}
	return md, nil
}

const (
	descStoragePercentDataTransferTime = "HP Enterprise Virtual Array storage controller metrics: Percentage CPU time used to perform data transfer operations."
	descStoragePercentProcessorTime    = "HP Enterprise Virtual Array storage controller metrics: Percentage CPU time."
)

type win32PerfRawDataEVAPMEXTHPEVAStorageController struct {
	Name                    string
	PercentDataTransferTime uint16 // HP Enterprise Virtual Array storage controller metrics: Percentage CPU time
	PercentProcessorTime    uint16 // HP Enterprise Virtual Array storage controller metrics: Percentage CPU time used to perform data transfer operations
}

func cHPEvaStorageArrayWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAStorageArray
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.storagearray.totalkb", v.TotalhostKBPers*1024, opentsdb.TagSet{"array": v.Name}, metadata.Counter, metadata.BytesPerSecond, descStorageTotalhostKBPers)
		Add(&md, "hp.eva.storagearray.totalrequests", v.TotalhostReqPers, opentsdb.TagSet{"array": v.Name}, metadata.Counter, metadata.PerSecond, descStorageTotalhostReqPers)
	}
	return md, nil
}

const (
	descStorageTotalhostKBPers  = "HP Enterprise Virtual Array general metrics: The total number of host requests in KBytes per second."
	descStorageTotalhostReqPers = "HP Enterprise Virtual Array general metrics: The total number of host requests per second."
)

type win32PerfRawDataEVAPMEXTHPEVAStorageArray struct {
	Name             string
	TotalhostKBPers  uint64 // HP Enterprise Virtual Array general metrics: The total number of host requests in KBytes per second
	TotalhostReqPers uint32 // HP Enterprise Virtual Array general metrics: The total number of host requests per second
}

func cHPEvaHostPortWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAHostPortStatistics
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.hostport.queue", v.AvQueueDepth, opentsdb.TagSet{"port": v.Name}, metadata.Gauge, metadata.Count, descHostPortAvQueueDepth)
		Add(&md, "hp.eva.hostport.bytes", v.ReadKBPers*1024, opentsdb.TagSet{"port": v.Name, "type": "read"}, metadata.Counter, metadata.BytesPerSecond, descHostPortKBPers)
		Add(&md, "hp.eva.hostport.bytes", v.WriteKBPers*1024, opentsdb.TagSet{"port": v.Name, "type": "write"}, metadata.Counter, metadata.BytesPerSecond, descHostPortKBPers)
		Add(&md, "hp.eva.hostport.ops", v.ReadReqPers, opentsdb.TagSet{"port": v.Name, "type": "read"}, metadata.Counter, metadata.PerSecond, descHostPortReqPers)
		Add(&md, "hp.eva.hostport.ops", v.WriteReqPers, opentsdb.TagSet{"port": v.Name, "type": "write"}, metadata.Counter, metadata.PerSecond, descHostPortReqPers)
		Add(&md, "hp.eva.hostport.latency", float64(v.ReadLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"port": v.Name, "type": "read"}, metadata.Gauge, metadata.MilliSecond, descHostPortLatencyus)
		Add(&md, "hp.eva.hostport.latency", float64(v.WriteLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"port": v.Name, "type": "write"}, metadata.Gauge, metadata.MilliSecond, descHostPortLatencyus)
	}
	return md, nil
}

const (
	descHostPortAvQueueDepth = "HP EVA Host port statistics: Average queue depth."
	descHostPortKBPers       = "HP EVA Host port statistics: Throughput in Bytes per second."
	descHostPortReqPers      = "HP EVA Host port statistics: Number of requests per second."
	descHostPortLatencyus    = "HP EVA Host port statistics: Latency in milliseconds."
)

type win32PerfRawDataEVAPMEXTHPEVAHostPortStatistics struct {
	Name           string
	AvQueueDepth   uint16 // HP EVA Host port statistics: Average queue depth
	ReadKBPers     uint64 // HP EVA Host port statistics: Read rate in KBytes per second
	ReadLatencyus  uint64 // HP EVA Host port statistics: Read latency in microseconds
	ReadReqPers    uint64 // HP EVA Host port statistics: Number of read requests per second
	WriteKBPers    uint64 // HP EVA Host port statistics: Write rate in KBytes per second
	WriteLatencyus uint64 // HP EVA Host port statistics: Write latency in microseconds
	WriteReqPers   uint64 // HP EVA Host port statistics: Number of write requests per second
}

func cHPEvaPhysicalDiskGroupWindows() (opentsdb.MultiDataPoint, error) {
	var dst []win32PerfRawDataEVAPMEXTHPEVAPhysicalDiskGroup
	var q = wmi.CreateQuery(&dst, ``)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "hp.eva.diskgroup.latency", float64(v.DriveLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"diskgroup": v.Name}, metadata.Gauge, metadata.MilliSecond, descPhysicalDiskDriveLatencyus)
		Add(&md, "hp.eva.diskgroup.queue", v.DriveQueueDepth, opentsdb.TagSet{"diskgroup": v.Name}, metadata.Gauge, metadata.Count, descPhysicalDiskDriveQueueDepth)
		Add(&md, "hp.eva.diskgroup.bytes", v.ReadKBPers*1024, opentsdb.TagSet{"diskgroup": v.Name, "type": "read"}, metadata.Counter, metadata.BytesPerSecond, descPhysicalDiskKBPers)
		Add(&md, "hp.eva.diskgroup.bytes", v.WriteKBPers*1024, opentsdb.TagSet{"diskgroup": v.Name, "type": "write"}, metadata.Counter, metadata.BytesPerSecond, descPhysicalDiskKBPers)
		Add(&md, "hp.eva.diskgroup.ops", v.ReadReqPers, opentsdb.TagSet{"diskgroup": v.Name, "type": "read"}, metadata.Counter, metadata.PerSecond, descPhysicalDiskReqPers)
		Add(&md, "hp.eva.diskgroup.ops", v.WriteReqPers, opentsdb.TagSet{"diskgroup": v.Name, "type": "write"}, metadata.Counter, metadata.PerSecond, descPhysicalDiskReqPers)
		Add(&md, "hp.eva.diskgroup.latency", float64(v.ReadLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"diskgroup": v.Name, "type": "read"}, metadata.Gauge, metadata.MilliSecond, descPhysicalDiskLatencyus)
		Add(&md, "hp.eva.diskgroup.latency", float64(v.WriteLatencyus)/hpEvaDisk1000nS1mS, opentsdb.TagSet{"diskgroup": v.Name, "type": "write"}, metadata.Gauge, metadata.MilliSecond, descPhysicalDiskLatencyus)
	}
	return md, nil
}

const (
	descPhysicalDiskDriveLatencyus  = "HP EVA Physical Disk Group performance data: Average drive latency in milliseconds. EVA GL only."
	descPhysicalDiskDriveQueueDepth = "HP EVA Physical Disk Group performance data: Average depth of the drive queue."
	descPhysicalDiskKBPers          = "HP EVA Physical Disk Group performance data: Throughput in Bytes per second."
	descPhysicalDiskReqPers         = "HP EVA Physical Disk Group performance data: Average requests per second."
	descPhysicalDiskLatencyus       = "HP EVA Physical Disk Group performance data: Average latency in milliseconds. EVA XL only."
)

type win32PerfRawDataEVAPMEXTHPEVAPhysicalDiskGroup struct {
	Name            string
	DriveLatencyus  uint64 // HP EVA Physical Disk Group performance data: Average drive latency in microseconds. EVA GL only
	DriveQueueDepth uint16 // HP EVA Physical Disk Group performance data: Average depth of the drive queue
	ReadKBPers      uint64 // HP EVA Physical Disk Group performance data: Average read in KBytes per second
	ReadLatencyus   uint64 // HP EVA Physical Disk Group performance data: Average read ilatency in microseconds. EVA XL only
	ReadReqPers     uint64 // HP EVA Physical Disk Group performance data: Average read requests per second
	WriteKBPers     uint64 // HP EVA Physical Disk Group performance data: Average write ilatency in microseconds. EVA XL only
	WriteLatencyus  uint64 // HP EVA Physical Disk Group performance data: Writes in KBytes per second
	WriteReqPers    uint64 // HP EVA Physical Disk Group performance data: Write requests per second
}
