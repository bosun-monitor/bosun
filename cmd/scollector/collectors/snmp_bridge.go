package collectors

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

const (
	vtpVlanState         = "1.3.6.1.4.1.9.9.46.1.3.1.1.2.1"
	dot1dTpFdbAddress    = "1.3.6.1.2.1.17.4.3.1.1"
	dot1dTpFdbPort       = "1.3.6.1.2.1.17.4.3.1.2"
	dot1dBasePortIfIndex = "1.3.6.1.2.1.17.1.4.1.2"
)

// SNMP Bridge registers
func SNMPBridge(cfg conf.SNMP) error {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_bridge(cfg.Community, cfg.Host)
		},
		Interval: time.Minute * 5,
		name:     fmt.Sprintf("snmp-bridge-%s", cfg.Host),
	})
	return nil
}

func c_snmp_bridge(community, host string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	vlanRaw, err := snmp_subtree(host, community, vtpVlanState)
	if err != nil {
		return md, err
	}
	vlans := []string{}
	for vlan, state := range vlanRaw {
		Add(&md, "cisco.net.vlan_state", state, opentsdb.TagSet{"host": host, "vlan": vlan}, metadata.Gauge, metadata.StatusCode, "")
		vlans = append(vlans, vlan)
	}
	ifMacs := make(map[string][]string)
	for _, vlan := range vlans {
		// community string indexing: http://www.cisco.com/c/en/us/support/docs/ip/simple-network-management-protocol-snmp/40367-camsnmp40367.html
		macRaw, err := snmp_subtree(host, community+"@"+vlan, dot1dTpFdbAddress)
		if err != nil {
			slog.Infoln(err)
			// continue since it might just be the one vlan
			continue
		}
		remoteMacAddresses := make(map[string]string)
		for k, v := range macRaw {
			if ba, ok := v.([]byte); ok {
				remoteMacAddresses[k] = strings.ToUpper(hex.EncodeToString(ba))
			}
		}
		toPort := make(map[string]string)
		toPortRaw, err := snmp_subtree(host, community+"@"+vlan, dot1dTpFdbPort)
		if err != nil {
			slog.Infoln(err)
		}
		for k, v := range toPortRaw {
			toPort[k] = fmt.Sprintf("%v", v)
		}
		portToIfIndex := make(map[string]string)
		portToIfIndexRaw, err := snmp_subtree(host, community+"@"+vlan, dot1dBasePortIfIndex)
		for k, v := range portToIfIndexRaw {
			portToIfIndex[k] = fmt.Sprintf("%v", v)
		}
		if err != nil {
			slog.Infoln(err)
		}
		for port, mac := range remoteMacAddresses {
			if port, ok := toPort[port]; ok {
				if ifIndex, ok := portToIfIndex[port]; ok {
					if _, ok := ifMacs[ifIndex]; ok {
						ifMacs[ifIndex] = append(ifMacs[ifIndex], mac)
					} else {
						ifMacs[ifIndex] = []string{mac}
					}
				}
			}
		}
	}
	for iface, macs := range ifMacs {
		for i, mac := range macs {
			metadata.AddMeta("", opentsdb.TagSet{"host": host, "iface": iface}, "remoteMac"+fmt.Sprintf("%v", i), mac, false)
		}
	}
	return md, nil
}
