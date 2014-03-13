package collectors

import (
	"fmt"
	"strings"
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

//IFXTable Constants
const (
	ifName               = ".1.3.6.1.2.1.31.1.1.1.1"
	ifAlias              = ".1.3.6.1.2.1.31.1.1.1.18"
	ifHCinOctets         = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCInUcastPkts      = ".1.3.6.1.2.1.31.1.1.1.7"
	ifHCInMulticastPkts  = ".1.3.6.1.2.1.31.1.1.1.8"
	ifHCInBroadcastPkts  = ".1.3.6.1.2.1.31.1.1.1.9"
	ifHCOutOctets        = ".1.3.6.1.2.1.31.1.1.1.10"
	ifHCOutUcastPkts     = ".1.3.6.1.2.1.31.1.1.1.11"
	ifHCOutMulticastPkts = ".1.3.6.1.2.1.31.1.1.1.12"
	ifHCOutBroadcastPkts = ".1.3.6.1.2.1.31.1.1.1.13"
)

// SNMPIfaces registers a SNMP Interfaces collector for the given community and host.
func SNMPIfaces(community, host string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() opentsdb.MultiDataPoint {
			return c_snmp_ifaces(community, host)
		},
		Interval: time.Minute * 5,
		name:     fmt.Sprintf("snmp-ifaces-%s", host),
	})
}

func switch_bond(metric, iname string) string {
	if strings.Contains(iname, "port-channel") {
		return "os.net.bond" + strings.TrimPrefix(metric, "os.net")
	}
	return metric
}

func c_snmp_ifaces(community, host string) opentsdb.MultiDataPoint {
	n, err := snmp_subtree(host, community, IfDescr)
	if err != nil {
		slog.Errorln("snmp ifaces1 :", err)
		return nil
	}
	a, err := snmp_subtree(host, community, ifAlias)
	if err != nil {
		slog.Errorln("snmp ifaces1 :", err)
		return nil
	}
	names := make(map[interface{}]string, len(n))
	aliases := make(map[interface{}]string, len(a))
	for k, v := range n {
		names[k] = fmt.Sprintf("%s", v)
	}
	for k, v := range a {
		aliases[k] = fmt.Sprintf("%s", v)
		if aliases[k] == "" {
			aliases[k] = "NA"
		}
	}
	var md opentsdb.MultiDataPoint
	add := func(oid, metric, dir string) error {
		m, err := snmp_subtree(host, community, oid)
		if err != nil {
			return err
		}
		for k, v := range m {
			Add(&md, switch_bond(metric, names[k]), v, opentsdb.TagSet{
				"host":      host,
				"direction": dir,
				"iface":     fmt.Sprint(k),
				"iname":     names[k],
				"alias":     aliases[k],
			})
		}
		return nil
	}
	oids := []snmpAdd{
		{ifHCinOctets, osNetBytes, "in"},
		{ifHCInUcastPkts, osNetUnicast, "in"},
		{ifHCInBroadcastPkts, osNetBroadcast, "in"},
		{ifHCInMulticastPkts, osNetMulticast, "in"},
		{ifInDiscards, osNetDropped, "in"},
		{ifInErrors, osNetErrors, "in"},
		{ifHCOutOctets, osNetBytes, "out"},
		{ifHCOutUcastPkts, osNetUnicast, "out"},
		{ifHCOutBroadcastPkts, osNetBroadcast, "out"},
		{ifHCOutMulticastPkts, osNetMulticast, "out"},
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
