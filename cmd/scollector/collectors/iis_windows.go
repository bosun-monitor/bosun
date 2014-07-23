package collectors

import (
	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	c := &IntervalCollector{
		F: c_iis_webservice,
	}
	c.init = wmiInit(c, func() interface{} { return &[]Win32_PerfRawData_W3SVC_WebService{} }, `WHERE Name <> '_Total'`, &iisQuery)
	collectors = append(collectors, c)
}

var (
	iisQuery string
)

func c_iis_webservice() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_W3SVC_WebService
	err := queryWmi(iisQuery, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "iis.bytes", v.BytesReceivedPersec, opentsdb.TagSet{"site": v.Name, "direction": "received"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.bytes", v.BytesSentPersec, opentsdb.TagSet{"site": v.Name, "direction": "sent"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.CGIRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "cgi"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.connection_attempts", v.ConnectionAttemptsPersec, opentsdb.TagSet{"site": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.CopyRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "copy"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.connections", v.CurrentConnections, opentsdb.TagSet{"site": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.DeleteRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "delete"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.GetRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "get"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.HeadRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "head"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.ISAPIExtensionRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "isapi"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.errors", v.LockedErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "locked"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.LockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "lock"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.MkcolRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "mkcol"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.MoveRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "move"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.errors", v.NotFoundErrorsPersec, opentsdb.TagSet{"site": v.Name, "type": "notfound"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.OptionsRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "options"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.PostRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "post"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.PropfindRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "propfind"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.ProppatchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "proppatch"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.PutRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "put"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.SearchRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "search"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.TraceRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "trace"}, metadata.Unknown, metadata.None, "")
		Add(&md, "iis.requests", v.UnlockRequestsPersec, opentsdb.TagSet{"site": v.Name, "method": "unlock"}, metadata.Unknown, metadata.None, "")
	}
	return md, nil
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
