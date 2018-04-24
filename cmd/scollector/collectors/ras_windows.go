package collectors

import (
	"fmt"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	c := &IntervalCollector{
		F: c_remote_access_services,
		//TODO: uncomment when done deving Interval: time.Minute,
	}
	c.init = func() {
		dst := []Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc{}
		query := wmi.CreateQuery(&dst, "")
		err := queryWmi(query, &dst)
		if err != nil {
			fmt.Println(err)
		}
		// If RaMgmtSvc and has at least one entry - we monitor it
		c.Enable = func() bool { return err == nil && len(dst) > 0 }
	}
	collectors = append(collectors, c)
}

const rasPrefix = "win.ras."

func c_remote_access_services() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_RemoteAccess_RASTotal
	query := wmi.CreateQuery(&dst, "")
	err := queryWmi(query, &dst)
	if err != nil {
		return nil, err
	}
	if len(dst) != 1 {
		return nil, fmt.Errorf("unexpected length for remote access (ras) total WMI query response, got %v expected 1", len(dst))
	}
	totals := dst[0]
	var md opentsdb.MultiDataPoint
	tags := opentsdb.TagSet{}
	Add(&md, rasPrefix+"total_connections", totals.TotalConnections, tags, metadata.Gauge, metadata.Connection, "TODO: DESC")
	return md, nil
}

// Win32_PerfRawData_RemoteAccess_RASTotal has aggregate stats for Windows Remote Access Services
// MSDN Reference https://msdn.microsoft.com/en-us/library/aa394330(v=vs.85).aspx
// Only one or zero rows (instances) are expected in a query reply
type Win32_PerfRawData_RemoteAccess_RASTotal struct {
	AlignmentErrors         uint32
	BufferOverrunErrors     uint32
	BytesReceived           uint64
	BytesReceivedPerSec     uint32
	BytesTransmitted        uint64
	BytesTransmittedPerSec  uint32
	CRCErrors               uint32
	FramesReceived          uint32
	FramesReceivedPerSec    uint32
	FramesTransmitted       uint32
	FramesTransmittedPerSec uint32
	Frequency_Object        uint64
	Frequency_PerfTime      uint64
	Frequency_Sys100NS      uint64
	PercentCompressionIn    uint32
	PercentCompressionOut   uint32
	SerialOverrunErrors     uint32
	TimeoutErrors           uint32
	Timestamp_Object        uint64
	Timestamp_PerfTime      uint64
	Timestamp_Sys100NS      uint64
	TotalConnections        uint32
	TotalErrors             uint32
	TotalErrorsPerSec       uint32
}

// Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc is used only to check for the existance of
// an active ras service on the host to decide if we want to monitor it. We are not interested
// in the actual values
type Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc struct {
	Timestamp_Sys100NS uint64
}
