package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, Collector{c_iis_webservice, DEFAULT_FREQ_SEC})
	//	collectors = append(collectors, c_iis_pool)
}

// KMB: Might be worth monitoring cache at
// Win32_PerfRawData_W3SVC_WebServiceCache, but the type isn't accessible via
// MSDN currently (getting Page Not Found).

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

type WorkerProcess struct {
	AppPoolName string
	Guid        string
	ProcessId   uint32
}

func c_iis_webservice() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_W3SVC_WebService
	q := CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := wmi.Query(q, &dst)
	if err != nil {
		l.Println("iis:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.webservice.bytes", v.BytesReceivedPersec, opentsdb.TagSet{"site": v.Name, "direction": "received"})
		Add(&md, "iis.webservice.bytes", v.BytesSentPersec, opentsdb.TagSet{"site": v.Name, "direction": "sent"})
		// Add(&md, "iis.webservice.requests", v.CGIRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "cgi"})
		// Add(&md, "iis.webservice.connection_attempts", v.ConnectionAttemptsPersec, opentsdb.TagSet{"site": v.Name})
		// Add(&md, "iis.webservice.requests", v.CopyRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "copy"})
		// Add(&md, "iis.webservice.connections", v.CurrentConnections, opentsdb.TagSet{"site": v.Name})
		// Add(&md, "iis.webservice.requests", v.DeleteRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "delete"})
		// Add(&md, "iis.webservice.requests", v.GetRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "get"})
		// Add(&md, "iis.webservice.requests", v.HeadRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "head"})
		// Add(&md, "iis.webservice.requests", v.ISAPIExtensionRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "isapi"})
		// Add(&md, "iis.webservice.errors", v.LockedErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "locked"})
		// Add(&md, "iis.webservice.requests", v.LockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "lock"})
		// Add(&md, "iis.webservice.requests", v.MkcolRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "mkcol"})
		// Add(&md, "iis.webservice.requests", v.MoveRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "move"})
		// Add(&md, "iis.webservice.errors", v.NotFoundErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "notfound"})
		// Add(&md, "iis.webservice.requests", v.OptionsRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "options"})
		// Add(&md, "iis.webservice.requests", v.PostRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "post"})
		// Add(&md, "iis.webservice.requests", v.PropfindRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "propfind"})
		// Add(&md, "iis.webservice.requests", v.ProppatchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "proppatch"})
		// Add(&md, "iis.webservice.requests", v.PutRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "put"})
		// Add(&md, "iis.webservice.requests", v.SearchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "search"})
		// Add(&md, "iis.webservice.requests", v.TraceRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "trace"})
		// Add(&md, "iis.webservice.requests", v.UnlockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "unlock"})

	}
	return md
}

// TODO Adding Most of these fields is crashing tcollector, not sure why
type Win32_PerfRawData_W3SVC_WebService struct {
	BytesReceivedPersec uint64
	BytesSentPersec     uint64
	// CGIRequestsPersec            uint32
	// ConnectionAttemptsPersec     uint32
	// CopyRequestsPersec           uint32
	// CurrentConnections           uint32
	// DeleteRequestsPersec         uint32
	// GetRequestsPersec            uint32
	// HeadRequestsPersec           uint32
	// ISAPIExtensionRequestsPersec uint32
	// LockedErrorsPersec           uint32
	// LockRequestsPersec           uint32
	// MkcolRequestsPersec          uint32
	// MoveRequestsPersec           uint32
	Name string
	// NotFoundErrorsPersec         uint32
	// OptionsRequestsPersec        uint32
	// PostRequestsPersec           uint32
	// PropfindRequestsPersec       uint32
	// ProppatchRequestsPersec      uint32
	// PutRequestsPersec            uint32
	// SearchRequestsPersec         uint32
	// TraceRequestsPersec          uint32
	// UnlockRequestsPersec         uint32
}
