package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_iis_webservice)
	collectors = append(collectors, c_iis_pool)
}

// KMB: Might be worth monitoring cache at
// Win32_PerfRawData_W3SVC_WebServiceCache, but the type isn't accessible via
// MSDN currently (getting Page Not Found).

const IIS_WEBSERVICE_QUERY = `
	SELECT 
		Name, 
		BytesReceivedPerSec, BytesSentPersec,
		ConnectionAttemptsPersec, CurrentConnections,
		CGIRequestsPersec, CopyRequestsPersec, DeleteRequestsPersec, 
		GetRequestsPersec, HeadRequestsPersec, ISAPIExtensionRequestsPersec, 
		LockRequestsPersec, MkcolRequestsPersec, MoveRequestsPersec, OptionsRequestsPersec,
		PostRequestsPersec, PropfindRequestsPersec, ProppatchRequestsPersec, PutRequestsPersec, 
		SearchRequestsPersec, TraceRequestsPersec, UnlockRequestsPersec,
		NotFoundErrorsPersec
	FROM Win32_PerfRawData_W3SVC_WebService
	WHERE Name <> '_Total'
`

const IIS_APOOL_QUERY = `
	SELECT AppPoolName, ProcessId
	From WorkerProcess
`

func c_iis_pool() opentsdb.MultiDataPoint {
	var dst []WorkerProcess
	err := wmi.Query(IIS_APOOL_QUERY, &dst) // should use namespace root\WebManagement
	if err != nil {
		l.Println("iis:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.apool.pid", v.ProcessId, opentsdb.TagSet{"name": v.AppPoolName})
	}
	return md
}

func c_iis_webservice() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_W3SVC_WebService
	err := wmi.Query(IIS_WEBSERVICE_QUERY, &dst)
	if err != nil {
		l.Println("iis:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.webservice.requests", v.CGIRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "cgi"})
		Add(&md, "iis.webservice.requests", v.CopyRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "copy"})
		Add(&md, "iis.webservice.requests", v.DeleteRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "delete"})
		Add(&md, "iis.webservice.requests", v.GetRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "get"})
		Add(&md, "iis.webservice.requests", v.HeadRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "head"})
		Add(&md, "iis.webservice.requests", v.ISAPIExtensionRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "isapi"})
		Add(&md, "iis.webservice.requests", v.LockRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "lock"})
		Add(&md, "iis.webservice.requests", v.MkcolRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "mkcol"})
		Add(&md, "iis.webservice.requests", v.MoveRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "move"})
		Add(&md, "iis.webservice.requests", v.OptionsRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "options"})
		Add(&md, "iis.webservice.requests", v.PostRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "post"})
		Add(&md, "iis.webservice.requests", v.PropfindRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "propfind"})
		Add(&md, "iis.webservice.requests", v.ProppatchRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "proppatch"})
		Add(&md, "iis.webservice.requests", v.PutRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "put"})
		Add(&md, "iis.webservice.requests", v.SearchRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "search"})
		Add(&md, "iis.webservice.requests", v.TraceRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "trace"})
		Add(&md, "iis.webservice.requests", v.UnlockRequestsPerSec, opentsdb.TagSet{"site": v.Name, "method": "unlock"})
		Add(&md, "iis.webservice.errors", v.LockedErrorsPerSec, opentsdb.TagSet{"site": v.Name, "type": "locked"})
		Add(&md, "iis.webservice.errors", v.NotFoundErrorsPerSec, opentsdb.TagSet{"site": v.Name, "type": "notfound"})
		Add(&md, "iis.webservice.bytes", v.BytesSentPerSec, opentsdb.TagSet{"site": v.Name, "direction": "sent"})
		Add(&md, "iis.webservice.bytes", v.BytesReceivedPerSec, opentsdb.TagSet{"site": v.Name, "direction": "received"})
		Add(&md, "iis.webservice.connection_attempts", v.ConnectionAttemptsPerSec, opentsdb.TagSet{"site": v.Name})
		Add(&md, "iis.webservice.connections", v.CurrentConnections, opentsdb.TagSet{"site": v.Name})
	}
	return md
}

type Win32_PerfRawData_W3SVC_WebService struct {
	BytesReceivedPerSec          uint64
	BytesSentPerSec              uint64
	CGIRequestsPerSec            uint32
	ConnectionAttemptsPerSec     uint32
	CopyRequestsPerSec           uint32
	CurrentConnections           uint32
	DeleteRequestsPerSec         uint32
	GetRequestsPerSec            uint32
	HeadRequestsPerSec           uint32
	ISAPIExtensionRequestsPerSec uint32
	LockedErrorsPerSec           uint32
	LockRequestsPerSec           uint32
	MkcolRequestsPerSec          uint32
	MoveRequestsPerSec           uint32
	Name                         string
	NotFoundErrorsPerSec         uint32
	OptionsRequestsPerSec        uint32
	PostRequestsPerSec           uint32
	PropfindRequestsPerSec       uint32
	ProppatchRequestsPerSec      uint32
	PutRequestsPerSec            uint32
	SearchRequestsPerSec         uint32
	TraceRequestsPerSec          uint32
	UnlockRequestsPerSec         uint32
}

type WorkerProcess struct {
	AppPoolName string
	Guid        string
	ProcessId   uint32
}
