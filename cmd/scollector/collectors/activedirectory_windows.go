package collectors

import (
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	c_ad := &IntervalCollector{
		F:        c_activedirectory_windows,
		Interval: time.Minute * 5,
	}
	c_ad.CollectorInit = wmiInitNamespace(c_ad, func() interface{} { return &[]MSAD_ReplNeighbor{} }, "", &adQuery, rootMSAD)
	collectors = append(collectors, c_ad)
}

var (
	adQuery  string
	rootMSAD = "root\\MicrosoftActiveDirectory"
)

func c_activedirectory_windows() (opentsdb.MultiDataPoint, error) {
	var dst []MSAD_ReplNeighbor
	err := queryWmiNamespace(adQuery, &dst, rootMSAD)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		lastSuccess, err := wmiParseCIMDatetime(v.TimeOfLastSyncSuccess)
		if err != nil {
			return nil, err
		}
		tags := opentsdb.TagSet{"source": strings.ToLower(v.SourceDsaCN), "context": activedirectory_context(v.NamingContextDN)}
		sinceLastSuccess := time.Now().Sub(lastSuccess).Seconds()
		Add(&md, "activedirectory.replication.sync_age", sinceLastSuccess, tags, metadata.Gauge, metadata.Second, descADReplicationSuccess)
		Add(&md, "activedirectory.replication.consecutive_failures", v.NumConsecutiveSyncFailures, tags, metadata.Gauge, metadata.Count, descADReplicationFailures)
	}
	return md, nil
}

const (
	descADReplicationSuccess  = "The number of seconds since the last successful replication attempt for this context."
	descADReplicationFailures = "The number of consecutive failed replication attempts for this context."
)

type MSAD_ReplNeighbor struct {
	SourceDsaCN                string
	NamingContextDN            string
	TimeOfLastSyncSuccess      string
	NumConsecutiveSyncFailures uint32
}

func activedirectory_context(NamingContextDN string) string {
	if strings.HasPrefix(NamingContextDN, "DC=DomainDnsZones,") {
		return "DomainDNSZones"
	}
	if strings.HasPrefix(NamingContextDN, "DC=ForestDnsZones,") {
		return "ForestDnsZones"
	}
	if strings.HasPrefix(NamingContextDN, "CN=Schema,CN=Configuration,") {
		return "Schema"
	}
	if strings.HasPrefix(NamingContextDN, "CN=Configuration,") {
		return "Configuration"
	}
	return "Domain"
}
