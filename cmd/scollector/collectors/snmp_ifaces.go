package collectors

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	ifAlias              = ".1.3.6.1.2.1.31.1.1.1.18"
	ifDescr              = ".1.3.6.1.2.1.2.2.1.2"
	ifType               = ".1.3.6.1.2.1.2.2.1.3"
	ifMTU                = ".1.3.6.1.2.1.2.2.1.4"
	ifPhysAddress        = ".1.3.6.1.2.1.2.2.1.6"
	ifHighSpeed          = ".1.3.6.1.2.1.31.1.1.1.15"
	ifAdminStatus        = ".1.3.6.1.2.1.2.2.1.7"
	ifOperStatus         = ".1.3.6.1.2.1.2.2.1.8"
	ifHCInBroadcastPkts  = ".1.3.6.1.2.1.31.1.1.1.9"
	ifHCInMulticastPkts  = ".1.3.6.1.2.1.31.1.1.1.8"
	ifHCInUcastPkts      = ".1.3.6.1.2.1.31.1.1.1.7"
	ifHCOutBroadcastPkts = ".1.3.6.1.2.1.31.1.1.1.13"
	ifHCOutMulticastPkts = ".1.3.6.1.2.1.31.1.1.1.12"
	ifHCOutOctets        = ".1.3.6.1.2.1.31.1.1.1.10"
	ifHCOutUcastPkts     = ".1.3.6.1.2.1.31.1.1.1.11"
	ifHCinOctets         = ".1.3.6.1.2.1.31.1.1.1.6"
	ifInDiscards         = ".1.3.6.1.2.1.2.2.1.13"
	ifInErrors           = ".1.3.6.1.2.1.2.2.1.14"
	ifInPauseFrames      = ".1.3.6.1.2.1.10.7.10.1.3"
	ifName               = ".1.3.6.1.2.1.31.1.1.1.1"
	ifOutDiscards        = ".1.3.6.1.2.1.2.2.1.19"
	ifOutErrors          = ".1.3.6.1.2.1.2.2.1.20"
	ifOutPauseFrames     = ".1.3.6.1.2.1.10.7.10.1.4"
)

// SNMPIfaces registers a SNMP Interfaces collector for the given community and host.
func SNMPIfaces(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_ifaces(cfg.Community, cfg.Host)
		},
		Interval: time.Second * 30,
		name:     fmt.Sprintf("snmp-ifaces-%s", cfg.Host),
	})
}

const osNet = "os.net"

func switchInterfaceMetric(metric string, iname string, ifType int64) string {
	switch ifType {
	case 6:
		return metric
	case 53, 161:
		return osNet + ".bond" + strings.TrimPrefix(metric, osNet)
	case 135:
		return osNet + ".virtual" + strings.TrimPrefix(metric, osNet)
	case 131:
		return osNet + ".tunnel" + strings.TrimPrefix(metric, osNet)
	default:
		//Cisco ASAs don't mark port channels correctly
		if strings.Contains(iname, "port-channel") {
			return osNet + ".bond" + strings.TrimPrefix(metric, osNet)
		}
		return osNet + ".other" + strings.TrimPrefix(metric, osNet)
	}
}

func c_snmp_ifaces(community, host string) (opentsdb.MultiDataPoint, error) {
	ifNamesRaw, err := snmp_subtree(host, community, ifName)
	if err != nil || len(ifNamesRaw) == 0 {
		ifNamesRaw, err = snmp_subtree(host, community, ifDescr)
		if err != nil {
			return nil, err
		}
	}
	ifAliasesRaw, err := snmp_subtree(host, community, ifAlias)
	if err != nil {
		return nil, err
	}
	ifTypesRaw, err := snmp_subtree(host, community, ifType)
	if err != nil {
		return nil, err
	}
	ifPhysAddressRaw, err := snmp_subtree(host, community, ifPhysAddress)
	if err != nil {
		return nil, err
	}
	ifNames := make(map[interface{}]string, len(ifNamesRaw))
	ifAliases := make(map[interface{}]string, len(ifAliasesRaw))
	ifTypes := make(map[interface{}]int64, len(ifTypesRaw))
	ifPhysAddresses := make(map[interface{}]string, len(ifPhysAddressRaw))
	for k, v := range ifNamesRaw {
		ifNames[k] = fmt.Sprintf("%s", v)
	}
	for k, v := range ifTypesRaw {
		val, ok := v.(int64)
		if !ok {
			return nil, fmt.Errorf("unexpected type from from MIB::ifType")
		}
		ifTypes[k] = val
	}
	for k, v := range ifAliasesRaw {
		// In case clean would come up empty, prevent the point from being removed
		// by setting our own empty case.
		ifAliases[k], _ = opentsdb.Clean(fmt.Sprintf("%s", v))
		if ifAliases[k] == "" {
			ifAliases[k] = "NA"
		}
	}
	for k, v := range ifPhysAddressRaw {
		ifPhysAddresses[k] = fmt.Sprintf("%X", v)
	}
	var md opentsdb.MultiDataPoint
	add := func(sA snmpAdd) error {
		m, err := snmp_subtree(host, community, sA.oid)
		if err != nil {
			return err
		}
		var sum int64
		for k, v := range m {
			tags := opentsdb.TagSet{
				"host":  host,
				"iface": fmt.Sprintf("%s", k),
				"iname": ifNames[k],
			}
			if sA.dir != "" {
				tags["direction"] = sA.dir
			}
			if iVal, ok := v.(int64); ok && ifTypes[k] == 6 {
				sum += iVal
			}
			Add(&md, switchInterfaceMetric(sA.metric, ifNames[k], ifTypes[k]), v, tags, sA.rate, sA.unit, sA.desc)
			metadata.AddMeta("", tags, "alias", ifAliases[k], false)
			metadata.AddMeta("", tags, "mac", ifPhysAddresses[k], false)
		}
		if sA.metric == OSNetBytes {
			tags := opentsdb.TagSet{"host": host, "direction": sA.dir}
			Add(&md, OSNetBytes+".total", sum, tags, metadata.Counter, metadata.Bytes, "The total number of bytes transfered through the network device.")
		}
		return nil
	}
	oids := []snmpAdd{
		{ifHCInBroadcastPkts, OSNetBroadcast, "in", metadata.Counter, metadata.Packet, OSNetBroadcastDesc},
		{ifHCInMulticastPkts, OSNetMulticast, "in", metadata.Counter, metadata.Packet, OSNetMulticastDesc},
		{ifHCInUcastPkts, OSNetUnicast, "in", metadata.Counter, metadata.Packet, OSNetUnicastDesc},
		{ifHCOutBroadcastPkts, OSNetBroadcast, "out", metadata.Counter, metadata.Packet, OSNetBroadcastDesc},
		{ifHCOutMulticastPkts, OSNetMulticast, "out", metadata.Counter, metadata.Packet, OSNetMulticastDesc},
		{ifHCOutOctets, OSNetBytes, "out", metadata.Counter, metadata.Bytes, OSNetBytesDesc},
		{ifHCOutUcastPkts, OSNetUnicast, "out", metadata.Counter, metadata.Packet, OSNetUnicastDesc},
		{ifHCinOctets, OSNetBytes, "in", metadata.Counter, metadata.Bytes, OSNetBytesDesc},
		{ifInDiscards, OSNetDropped, "in", metadata.Counter, metadata.Packet, OSNetDroppedDesc},
		{ifInErrors, OSNetErrors, "in", metadata.Counter, metadata.Error, OSNetErrorsDesc},
		{ifOutDiscards, OSNetDropped, "out", metadata.Counter, metadata.Packet, OSNetDroppedDesc},
		{ifOutErrors, OSNetErrors, "out", metadata.Counter, metadata.Error, OSNetErrorsDesc},
		{ifInPauseFrames, OSNetPauseFrames, "in", metadata.Counter, metadata.Frame, OSNetPauseFrameDesc},
		{ifOutPauseFrames, OSNetPauseFrames, "out", metadata.Counter, metadata.Frame, OSNetPauseFrameDesc},
		{ifMTU, OSNetMTU, "", metadata.Gauge, metadata.Bytes, OSNetMTUDesc},
		{ifHighSpeed, OSNetIfSpeed, "", metadata.Gauge, metadata.Megabit, OSNetIfSpeedDesc},
		{ifAdminStatus, OSNetAdminStatus, "", metadata.Gauge, metadata.StatusCode, OSNetAdminStatusDesc},
		{ifOperStatus, OSNetOperStatus, "", metadata.Gauge, metadata.StatusCode, OSNetOperStatusDesc},
	}
	for _, sA := range oids {
		if err := add(sA); err != nil {
			return nil, err
		}
	}
	return md, nil
}

type snmpAdd struct {
	oid    string
	metric string
	dir    string
	rate   metadata.RateType
	unit   metadata.Unit
	desc   string
}
