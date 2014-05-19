package collectors

import (
	"sync"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{
		F:    c_iis_webservice,
		init: wmiInit(&iisEnable, &iisLock, []Win32_PerfRawData_W3SVC_WebService{}, `WHERE Name <> '_Total'`, &iisQuery),
	})
}

var (
	iisEnable bool
	iisLock   sync.Mutex
	iisQuery  string
)

func iisEnabled() (b bool) {
	iisLock.Lock()
	b = iisEnable
	iisLock.Unlock()
	return
}

func c_iis_webservice() opentsdb.MultiDataPoint {
	if !iisEnabled() {
		return nil
	}
	var dst []Win32_PerfRawData_W3SVC_WebService
	err := queryWmi(iisQuery, &dst)
	if err != nil {
		slog.Infoln("iis:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.bytes", v.BytesReceivedPersec, opentsdb.TagSet{"site": v.Name, "direction": "received"})
		Add(&md, "iis.bytes", v.BytesSentPersec, opentsdb.TagSet{"site": v.Name, "direction": "sent"})
		Add(&md, "iis.requests", v.CGIRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "cgi"})
		Add(&md, "iis.connection_attempts", v.ConnectionAttemptsPersec, opentsdb.TagSet{"site": v.Name})
		Add(&md, "iis.requests", v.CopyRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "copy"})
		Add(&md, "iis.connections", v.CurrentConnections, opentsdb.TagSet{"site": v.Name})
		Add(&md, "iis.requests", v.DeleteRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "delete"})
		Add(&md, "iis.requests", v.GetRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "get"})
		Add(&md, "iis.requests", v.HeadRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "head"})
		Add(&md, "iis.requests", v.ISAPIExtensionRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "isapi"})
		Add(&md, "iis.errors", v.LockedErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "locked"})
		Add(&md, "iis.requests", v.LockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "lock"})
		Add(&md, "iis.requests", v.MkcolRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "mkcol"})
		Add(&md, "iis.requests", v.MoveRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "move"})
		Add(&md, "iis.errors", v.NotFoundErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "notfound"})
		Add(&md, "iis.requests", v.OptionsRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "options"})
		Add(&md, "iis.requests", v.PostRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "post"})
		Add(&md, "iis.requests", v.PropfindRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "propfind"})
		Add(&md, "iis.requests", v.ProppatchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "proppatch"})
		Add(&md, "iis.requests", v.PutRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "put"})
		Add(&md, "iis.requests", v.SearchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "search"})
		Add(&md, "iis.requests", v.TraceRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "trace"})
		Add(&md, "iis.requests", v.UnlockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "unlock"})
	}
	return md
}

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
