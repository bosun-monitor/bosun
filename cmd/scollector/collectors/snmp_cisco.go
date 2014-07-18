package collectors

import (
	"fmt"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

const (
	ciscoCPU     = ".1.3.6.1.4.1.9.9.109.1.1.1.1.6"
	ciscoMemFree = ".1.3.6.1.4.1.9.9.48.1.1.1.6"
	ciscoMemName = ".1.3.6.1.4.1.9.9.48.1.1.1.2"
	ciscoMemUsed = ".1.3.6.1.4.1.9.9.48.1.1.1.5"
)

// SNMPCisco registers a SNMP CISCO collector for the given community and host.
func SNMPCisco(community, host string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() opentsdb.MultiDataPoint {
			return c_snmp_cisco(community, host)
		},
		Interval: time.Second * 30,
		name:     fmt.Sprintf("snmp-cisco-%s", host),
	})
}

func c_snmp_cisco(community, host string) opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	if v, err := snmp_oid(host, community, ciscoCPU); err != nil {
		slog.Infoln("snmp cisco cpu:", err)
	} else {
		Add(&md, "cisco.cpu", v, opentsdb.TagSet{"host": host}, metadata.Unknown, metadata.None, "")
	}
	names, err := snmp_subtree(host, community, ciscoMemName)
	if err != nil {
		slog.Infoln("snmp cisco mem:", err)
	}
	used, err := snmp_subtree(host, community, ciscoMemUsed)
	if err != nil {
		slog.Infoln("snmp cisco mem:", err)
	}
	free, err := snmp_subtree(host, community, ciscoMemFree)
	if err != nil {
		slog.Infoln("snmp cisco mem:", err)
	}
	for id, name := range names {
		n := fmt.Sprintf("%s", name)
		u, present := used[id]
		if !present {
			continue
		}
		f, present := free[id]
		if !present {
			continue
		}
		Add(&md, "cisco.mem.used", u, opentsdb.TagSet{"host": host, "name": n}, metadata.Unknown, metadata.None, "")
		Add(&md, "cisco.mem.free", f, opentsdb.TagSet{"host": host, "name": n}, metadata.Unknown, metadata.None, "")
	}
	return md
}
