package collectors

import (
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	c := &IntervalCollector{
		F: c_iis_webservice,
	}
	c.CollectorInit = wmiInit(c, func() interface{} { return &[]Win32_PerfRawData_W3SVC_WebService{} }, `WHERE Name <> '_Total'`, &iisQuery)
	collectors = append(collectors, c)

	c = &IntervalCollector{
		F: c_iis_apppool,
	}
	c.CollectorInit = wmiInit(c, func() interface{} { return &[]Win32_PerfRawData_APPPOOLCountersProvider_APPPOOLWAS{} }, `WHERE Name <> '_Total'`, &iisQueryAppPool)
	collectors = append(collectors, c)
}

var (
	iisQuery        string
	iisQueryAppPool string
)

func c_iis_webservice() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_W3SVC_WebService
	err := queryWmi(iisQuery, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.bytes", v.BytesReceivedPersec, opentsdb.TagSet{"site": v.Name, "direction": "received"}, metadata.Counter, metadata.BytesPerSecond, descIISBytesReceivedPersec)
		Add(&md, "iis.bytes", v.BytesSentPersec, opentsdb.TagSet{"site": v.Name, "direction": "sent"}, metadata.Counter, metadata.BytesPerSecond, descIISBytesSentPersec)
		Add(&md, "iis.requests", v.CGIRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "cgi"}, metadata.Counter, metadata.PerSecond, descIISCGIRequestsPersec)
		Add(&md, "iis.connection_attempts", v.ConnectionAttemptsPersec, opentsdb.TagSet{"site": v.Name}, metadata.Counter, metadata.PerSecond, descIISConnectionAttemptsPersec)
		Add(&md, "iis.requests", v.CopyRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "copy"}, metadata.Counter, metadata.PerSecond, descIISCopyRequestsPersec)
		Add(&md, "iis.connections", v.CurrentConnections, opentsdb.TagSet{"site": v.Name}, metadata.Gauge, metadata.Count, descIISCurrentConnections)
		Add(&md, "iis.requests", v.DeleteRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "delete"}, metadata.Counter, metadata.PerSecond, descIISDeleteRequestsPersec)
		Add(&md, "iis.requests", v.GetRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "get"}, metadata.Counter, metadata.PerSecond, descIISGetRequestsPersec)
		Add(&md, "iis.requests", v.HeadRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "head"}, metadata.Counter, metadata.PerSecond, descIISHeadRequestsPersec)
		Add(&md, "iis.requests", v.ISAPIExtensionRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "isapi"}, metadata.Counter, metadata.PerSecond, descIISISAPIExtensionRequestsPersec)
		Add(&md, "iis.errors", v.LockedErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "locked"}, metadata.Counter, metadata.PerSecond, descIISLockedErrorsPersec)
		Add(&md, "iis.requests", v.LockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "lock"}, metadata.Counter, metadata.PerSecond, descIISLockRequestsPersec)
		Add(&md, "iis.requests", v.MkcolRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "mkcol"}, metadata.Counter, metadata.PerSecond, descIISMkcolRequestsPersec)
		Add(&md, "iis.requests", v.MoveRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "move"}, metadata.Counter, metadata.PerSecond, descIISMoveRequestsPersec)
		Add(&md, "iis.errors", v.NotFoundErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "notfound"}, metadata.Counter, metadata.PerSecond, descIISNotFoundErrorsPersec)
		Add(&md, "iis.requests", v.OptionsRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "options"}, metadata.Counter, metadata.PerSecond, descIISOptionsRequestsPersec)
		Add(&md, "iis.requests", v.PostRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "post"}, metadata.Counter, metadata.PerSecond, descIISPostRequestsPersec)
		Add(&md, "iis.requests", v.PropfindRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "propfind"}, metadata.Counter, metadata.PerSecond, descIISPropfindRequestsPersec)
		Add(&md, "iis.requests", v.ProppatchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "proppatch"}, metadata.Counter, metadata.PerSecond, descIISProppatchRequestsPersec)
		Add(&md, "iis.requests", v.PutRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "put"}, metadata.Counter, metadata.PerSecond, descIISPutRequestsPersec)
		Add(&md, "iis.requests", v.SearchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "search"}, metadata.Counter, metadata.PerSecond, descIISSearchRequestsPersec)
		Add(&md, "iis.requests", v.TraceRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "trace"}, metadata.Counter, metadata.PerSecond, descIISTraceRequestsPersec)
		Add(&md, "iis.requests", v.UnlockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "unlock"}, metadata.Counter, metadata.PerSecond, descIISUnlockRequestsPersec)
	}
	return md, nil
}

const (
	descIISBytesReceivedPersec          = "The rate that data bytes are received by the Web service."
	descIISBytesSentPersec              = "The rate data bytes are being sent by the Web service."
	descIISCGIRequestsPersec            = "The rate CGI requests are received by the Web service."
	descIISConnectionAttemptsPersec     = "The rate that connections to the Web service are being attempted."
	descIISCopyRequestsPersec           = "The rate HTTP requests using the COPY method are made.  Copy requests are used for copying files and directories."
	descIISCurrentConnections           = "The current number of connections established with the Web service."
	descIISDeleteRequestsPersec         = "The rate HTTP requests using the DELETE method are made.  Delete requests are generally used for file removals."
	descIISGetRequestsPersec            = "The rate HTTP requests using the GET method are made.  Get requests are the most common HTTP request."
	descIISHeadRequestsPersec           = "The rate HTTP requests using the HEAD method are made.  Head requests generally indicate a client is querying the state of a document they already have to see if it needs to be refreshed."
	descIISISAPIExtensionRequestsPersec = "The rate that ISAPI Extension requests are received by the Web service."
	descIISLockedErrorsPersec           = "The rate of errors due to requests that couldn't be satisfied by the server because the requested document was locked.  These are generally reported as an HTTP 423 error code to the client."
	descIISLockRequestsPersec           = "The rate HTTP requests using the LOCK method are made.  Lock requests are used to lock a file for one user so that only that user can modify the file."
	descIISMkcolRequestsPersec          = "The rate HTTP requests using the MKCOL method are made.  Mkcol requests are used to create directories on the server."
	descIISMoveRequestsPersec           = "The rate HTTP requests using the MOVE method are made.  Move requests are used for moving files and directories."
	descIISNotFoundErrorsPersec         = "The rate of errors due to requests that couldn't be satisfied by the server because the requested document could not be found.  These are generally reported as an HTTP 404 error code to the client."
	descIISOptionsRequestsPersec        = "The rate HTTP requests using the OPTIONS method are made."
	descIISPostRequestsPersec           = "The rate HTTP requests using the POST method are made."
	descIISPropfindRequestsPersec       = "The rate HTTP requests using the PROPFIND method are made.  Propfind requests retrieve property values on files and directories."
	descIISProppatchRequestsPersec      = "The rate HTTP requests using the PROPPATCH method are made.  Proppatch requests set property values on files and directories."
	descIISPutRequestsPersec            = "The rate HTTP requests using the PUT method are made."
	descIISSearchRequestsPersec         = "The rate HTTP requests using the SEARCH method are made.  Search requests are used to query the server to find resources that match a set of conditions provided by the client."
	descIISTraceRequestsPersec          = "The rate HTTP requests using the TRACE method are made.  Trace requests allow the client to see what is being received at the end of the request chain and use the information for diagnostic purposes."
	descIISUnlockRequestsPersec         = "The rate HTTP requests using the UNLOCK method are made.  Unlock requests are used to remove locks from files."
)

type Win32_PerfRawData_W3SVC_WebService struct {
	BytesReceivedPersec          uint64
	BytesSentPersec              uint64
	CGIRequestsPersec            uint32
	ConnectionAttemptsPersec     uint32
	CopyRequestsPersec           uint32
	CurrentConnections           uint32
	DeleteRequestsPersec         uint32
	GetRequestsPersec            uint32
	HeadRequestsPersec           uint32
	ISAPIExtensionRequestsPersec uint32
	LockRequestsPersec           uint32
	LockedErrorsPersec           uint32
	MkcolRequestsPersec          uint32
	MoveRequestsPersec           uint32
	Name                         string
	NotFoundErrorsPersec         uint32
	OptionsRequestsPersec        uint32
	PostRequestsPersec           uint32
	PropfindRequestsPersec       uint32
	ProppatchRequestsPersec      uint32
	PutRequestsPersec            uint32
	SearchRequestsPersec         uint32
	TraceRequestsPersec          uint32
	UnlockRequestsPersec         uint32
}

func c_iis_apppool() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_APPPOOLCountersProvider_APPPOOLWAS
	err := queryWmi(iisQueryAppPool, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		tags := opentsdb.TagSet{"name": v.Name}
		if v.Frequency_Object != 0 {
			uptime := (v.Timestamp_Object - v.CurrentApplicationPoolUptime) / v.Frequency_Object
			failtime := (v.Timestamp_Object - v.TimeSinceLastWorkerProcessFailure) / v.Frequency_Object
			Add(&md, "iis.apppool.uptime", uptime, tags, metadata.Gauge, metadata.Second, descIISAppPoolCurrentApplicationPoolUptime)
			Add(&md, "iis.apppool.time_since_failure", failtime, tags, metadata.Gauge, metadata.Second, descIISAppPoolTimeSinceLastWorkerProcessFailure)
		}
		Add(&md, "iis.apppool.state", v.CurrentApplicationPoolState, tags, metadata.Gauge, metadata.StatusCode, descIISAppPoolCurrentApplicationPoolState)
		Add(&md, "iis.apppool.processes", v.CurrentWorkerProcesses, opentsdb.TagSet{"name": v.Name, "type": "current"}, metadata.Gauge, metadata.Count, descIISAppPoolCurrentWorkerProcesses)
		Add(&md, "iis.apppool.processes", v.MaximumWorkerProcesses, opentsdb.TagSet{"name": v.Name, "type": "maximum"}, metadata.Gauge, metadata.Count, descIISAppPoolMaximumWorkerProcesses)
		Add(&md, "iis.apppool.processes", v.RecentWorkerProcessFailures, opentsdb.TagSet{"name": v.Name, "type": "failed"}, metadata.Gauge, metadata.Count, descIISAppPoolRecentWorkerProcessFailures)
		Add(&md, "iis.apppool.events", v.TotalApplicationPoolRecycles, opentsdb.TagSet{"name": v.Name, "type": "recycled"}, metadata.Counter, metadata.Event, descIISAppPoolTotalApplicationPoolRecycles)
		Add(&md, "iis.apppool.events", v.TotalWorkerProcessesCreated, opentsdb.TagSet{"name": v.Name, "type": "created"}, metadata.Counter, metadata.Event, descIISAppPoolTotalWorkerProcessesCreated)
		Add(&md, "iis.apppool.events", v.TotalWorkerProcessFailures, opentsdb.TagSet{"name": v.Name, "type": "failed_crash"}, metadata.Counter, metadata.Event, descIISAppPoolTotalWorkerProcessFailures)
		Add(&md, "iis.apppool.events", v.TotalWorkerProcessPingFailures, opentsdb.TagSet{"name": v.Name, "type": "failed_ping"}, metadata.Counter, metadata.Event, descIISAppPoolTotalWorkerProcessPingFailures)
		Add(&md, "iis.apppool.events", v.TotalWorkerProcessShutdownFailures, opentsdb.TagSet{"name": v.Name, "type": "failed_shutdown"}, metadata.Counter, metadata.Event, descIISAppPoolTotalWorkerProcessShutdownFailures)
		Add(&md, "iis.apppool.events", v.TotalWorkerProcessStartupFailures, opentsdb.TagSet{"name": v.Name, "type": "failed_startup"}, metadata.Counter, metadata.Event, descIISAppPoolTotalWorkerProcessStartupFailures)
	}
	return md, nil
}

const (
	descIISAppPoolCurrentApplicationPoolState        = "The current status of the application pool (1 - Uninitialized, 2 - Initialized, 3 - Running, 4 - Disabling, 5 - Disabled, 6 - Shutdown Pending, 7 - Delete Pending)."
	descIISAppPoolCurrentApplicationPoolUptime       = "The length of time, in seconds, that the application pool has been running since it was started."
	descIISAppPoolCurrentWorkerProcesses             = "The current number of worker processes that are running in the application pool."
	descIISAppPoolMaximumWorkerProcesses             = "The maximum number of worker processes that have been created for the application pool since Windows Process Activation Service (WAS) started."
	descIISAppPoolRecentWorkerProcessFailures        = "The number of times that worker processes for the application pool failed during the rapid-fail protection interval."
	descIISAppPoolTimeSinceLastWorkerProcessFailure  = "The length of time, in seconds, since the last worker process failure occurred for the application pool."
	descIISAppPoolTotalApplicationPoolRecycles       = "The number of times that the application pool has been recycled since Windows Process Activation Service (WAS) started."
	descIISAppPoolTotalWorkerProcessesCreated        = "The number of worker processes created for the application pool since Windows Process Activation Service (WAS) started."
	descIISAppPoolTotalWorkerProcessFailures         = "The number of times that worker processes have crashed since the application pool was started."
	descIISAppPoolTotalWorkerProcessPingFailures     = "The number of times that Windows Process Activation Service (WAS) did not receive a response to ping messages sent to a worker process."
	descIISAppPoolTotalWorkerProcessShutdownFailures = "The number of times that Windows Process Activation Service (WAS) failed to shut down a worker process."
	descIISAppPoolTotalWorkerProcessStartupFailures  = "The number of times that Windows Process Activation Service (WAS) failed to start a worker process."
)

type Win32_PerfRawData_APPPOOLCountersProvider_APPPOOLWAS struct {
	CurrentApplicationPoolState        uint32
	CurrentApplicationPoolUptime       uint64
	CurrentWorkerProcesses             uint32
	Frequency_Object                   uint64
	MaximumWorkerProcesses             uint32
	Name                               string
	RecentWorkerProcessFailures        uint32
	TimeSinceLastWorkerProcessFailure  uint64
	Timestamp_Object                   uint64
	TotalApplicationPoolRecycles       uint32
	TotalWorkerProcessesCreated        uint32
	TotalWorkerProcessFailures         uint32
	TotalWorkerProcessPingFailures     uint32
	TotalWorkerProcessShutdownFailures uint32
	TotalWorkerProcessStartupFailures  uint32
}
