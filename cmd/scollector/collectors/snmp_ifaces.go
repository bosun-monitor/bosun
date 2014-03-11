package collectors

import (
	"fmt"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

const (
	IfDescr         = ".1.3.6.1.2.1.2.2.1.2"
	ifInOctets      = ".1.3.6.1.2.1.2.2.1.10"
	ifInUcastPkts   = ".1.3.6.1.2.1.2.2.1.11"
	ifInNUcastPkts  = ".1.3.6.1.2.1.2.2.1.12"
	ifInDiscards    = ".1.3.6.1.2.1.2.2.1.13"
	ifInErrors      = ".1.3.6.1.2.1.2.2.1.14"
	ifOutOctets     = ".1.3.6.1.2.1.2.2.1.16"
	ifOutUcastPkts  = ".1.3.6.1.2.1.2.2.1.17"
	ifOutNUcastPkts = ".1.3.6.1.2.1.2.2.1.18"
	ifOutDiscards   = ".1.3.6.1.2.1.2.2.1.19"
	ifOutErrors     = ".1.3.6.1.2.1.2.2.1.20"
)

// SNMPIfaces registers a SNMP Interfaces collector for the given community and host.
func SNMPIfaces(community, host string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() opentsdb.MultiDataPoint {
			return c_snmp_ifaces(community, host)
		},
		Interval: time.Minute * 5,
	})
}

func c_snmp_ifaces(community, host string) opentsdb.MultiDataPoint {
	n, err := snmp_subtree(host, community, IfDescr)
	if err != nil {
		slog.Errorln("snmp ifaces1 :", err)
		return nil
	}
	names := make(map[interface{}]string, len(n))
	for k, v := range n {
		names[k] = fmt.Sprintf("%s", v)
	}
	var md opentsdb.MultiDataPoint
	add := func(oid, metric, dir string) error {
		m, err := snmp_subtree(host, community, oid)
		if err != nil {
			return err
		}
		for k, v := range m {
			Add(&md, metric, v, opentsdb.TagSet{
				"host":      host,
				"direction": dir,
				"iface":     fmt.Sprint(k),
				"iname":     names[k],
			})
		}
		return nil
	}
	oids := []snmpAdd{
		{ifInOctets, osNetBytes, "in"},
		{ifInUcastPkts, osNetUnicast, "in"},
		{ifInNUcastPkts, osNetBroadcast, "in"},
		{ifInDiscards, osNetDropped, "in"},
		{ifInErrors, osNetErrors, "in"},
		{ifOutOctets, osNetBytes, "out"},
		{ifOutUcastPkts, osNetUnicast, "out"},
		{ifOutNUcastPkts, osNetBroadcast, "out"},
		{ifOutDiscards, osNetDropped, "out"},
		{ifOutErrors, osNetErrors, "out"},
	}
	for _, o := range oids {
		if err := add(o.oid, o.metric, o.dir); err != nil {
			slog.Errorln("snmp:", err)
			return nil
		}
	}
	return md
}

type snmpAdd struct {
	oid    string
	metric string
	dir    string
}
