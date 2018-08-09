package collectors

import (
	"fmt"
	"time"

	"bosun.org/collect"
	"bosun.org/slog"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	c := &IntervalCollector{
		F:        c_remote_access_services,
		Interval: time.Minute,
	}
	dst := []Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc{}
	query := wmi.CreateQuery(&dst, "")
	err := queryWmi(query, &dst)
	if err != nil && collect.Debug {
		slog.Error(err)
	}
	// If RaMgmtSvc and has at least one entry - we monitor it
	if err == nil && len(dst) > 0 {
		collectors = append(collectors, c)
	}
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
	ts := TSys100NStoEpoch(totals.Timestamp_Sys100NS)
	Add(&md, rasPrefix+"total_connections", totals.TotalConnections, tags, metadata.Gauge, metadata.Connection, descWinRasTotalConnections)
	AddTS(&md, rasPrefix+"total_bytes", ts, totals.BytesReceived, opentsdb.TagSet{"direction": "in"}, metadata.Counter, metadata.Bytes, descWinRasTotalBytes)
	AddTS(&md, rasPrefix+"total_bytes", ts, totals.BytesTransmitted, opentsdb.TagSet{"direction": "out"}, metadata.Counter, metadata.Bytes, descWinRasTotalBytes)
	AddTS(&md, rasPrefix+"total_frames", ts, totals.FramesReceived, opentsdb.TagSet{"direction": "in"}, metadata.Counter, metadata.Frame, descWinRasFrames)
	AddTS(&md, rasPrefix+"total_frames", ts, totals.FramesTransmitted, opentsdb.TagSet{"direction": "out"}, metadata.Counter, metadata.Frame, descWinRasFrames)
	AddTS(&md, rasPrefix+"total_errors_by_type", ts, totals.AlignmentErrors, opentsdb.TagSet{"type": "alignment"}, metadata.Counter, metadata.Error, descWinRasErrorByType)
	AddTS(&md, rasPrefix+"total_errors_by_type", ts, totals.BufferOverrunErrors, opentsdb.TagSet{"type": "buffer_overrun"}, metadata.Counter, metadata.Error, descWinRasErrorByType)
	AddTS(&md, rasPrefix+"total_errors_by_type", ts, totals.CRCErrors, opentsdb.TagSet{"type": "crc"}, metadata.Counter, metadata.Error, descWinRasErrorByType)
	AddTS(&md, rasPrefix+"total_errors_by_type", ts, totals.SerialOverrunErrors, opentsdb.TagSet{"type": "serial_overrun"}, metadata.Counter, metadata.Error, descWinRasErrorByType)
	AddTS(&md, rasPrefix+"total_errors_by_type", ts, totals.TimeoutErrors, opentsdb.TagSet{"type": "timeout"}, metadata.Counter, metadata.Error, descWinRasErrorByType)
	AddTS(&md, rasPrefix+"total_errors", ts, totals.TotalErrors, tags, metadata.Counter, metadata.Error, descWinRasTotalErrors)
	return md, nil
}

// Win32_PerfRawData_RemoteAccess_RASTotal has aggregate stats for Windows Remote Access Services
// MSDN Reference https://msdn.microsoft.com/en-us/library/aa394330(v=vs.85).aspx
// Only one or zero rows (instances) are expected in a query reply
type Win32_PerfRawData_RemoteAccess_RASTotal struct {
	AlignmentErrors     uint32
	BufferOverrunErrors uint32
	BytesReceived       uint64
	BytesTransmitted    uint64
	CRCErrors           uint32
	FramesReceived      uint32
	FramesTransmitted   uint32
	Frequency_Sys100NS  uint64
	SerialOverrunErrors uint32
	TimeoutErrors       uint32
	Timestamp_Sys100NS  uint64
	TotalConnections    uint32
	TotalErrors         uint32
}

const (
	descWinRasTotalConnections = "The total number of current Remote Access connections."
	descWinRasTotalBytes       = "The total number of bytes transmitted and received for Remote Access connections."
	descWinRasFrames           = "The total number of frames transmitted and received for Remote Access connections."
	descWinRasErrorByType      = "The total number of errors on Remote Access connections by type: Alignment Errors occur when a byte received is different from the byte expected.  Buffer Overrun Errors when the software cannot handle the rate at which data is received. CRC Errors occur when the frame received contains erroneous data. Serial Overrun Errors occur when the hardware cannot handle the rate at which data is received. Timeout Errors occur when an expected is not received in time."
	descWinRasTotalErrors      = "The total number of errors on Remote Access connections."
)

// Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc is used only to check for the existence of
// an active ras service on the host to decide if we want to monitor it. We are not interested
// in the actual values
type Win32_PerfRawData_RamgmtSvcCounterProvider_RaMgmtSvc struct {
	Timestamp_Sys100NS uint64
}
