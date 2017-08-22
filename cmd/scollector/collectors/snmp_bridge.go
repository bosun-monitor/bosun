package collectors

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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

// SNMPBridge registers
func SNMPBridge(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return cSnmpBridge(cfg.Community, cfg.Host)
		},
		Interval: time.Minute * 5,
		name:     fmt.Sprintf("snmp-bridge-%s", cfg.Host),
	})
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return cSnmpCdp(cfg.Community, cfg.Host)
		},
		Interval: time.Minute * 5,
		name:     fmt.Sprintf("snmp-cdp-%s", cfg.Host),
	})
}

func cSnmpBridge(community, host string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	vlanRaw, err := snmpSubtree(host, community, vtpVlanState)
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
		macRaw, err := snmpSubtree(host, community+"@"+vlan, dot1dTpFdbAddress)
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
		toPortRaw, err := snmpSubtree(host, community+"@"+vlan, dot1dTpFdbPort)
		if err != nil {
			slog.Infoln(err)
		}
		for k, v := range toPortRaw {
			toPort[k] = fmt.Sprintf("%v", v)
		}
		portToIfIndex := make(map[string]string)
		portToIfIndexRaw, err := snmpSubtree(host, community+"@"+vlan, dot1dBasePortIfIndex)
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
		sort.Strings(macs)
		j, err := json.Marshal(macs)
		if err != nil {
			return md, nil
		}
		metadata.AddMeta("", opentsdb.TagSet{"host": host, "iface": iface}, "remoteMacs", string(j), false)
	}
	return md, nil
}

const (
	cdpCacheDeviceID   = "1.3.6.1.4.1.9.9.23.1.2.1.1.6"
	cdpCacheDevicePort = "1.3.6.1.4.1.9.9.23.1.2.1.1.7"
)

type cdpCacheEntry struct {
	InterfaceID string `json:"-"`
	DeviceID    string
	DevicePort  string
}

func cSnmpCdp(community, host string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	cdpEntries := make(map[string]*cdpCacheEntry)
	deviceIDRaw, err := snmpSubtree(host, community, cdpCacheDeviceID)
	if err != nil {
		return md, err
	}
	for k, v := range deviceIDRaw {
		ids := strings.Split(k, ".")
		if len(ids) != 2 {
			slog.Error("unexpected snmp cdpCacheEntry id")
			continue
		}
		cdpEntries[ids[0]] = &cdpCacheEntry{}
		cdpEntries[ids[0]].DeviceID = fmt.Sprintf("%s", v)
		cdpEntries[ids[0]].InterfaceID = ids[1]
	}
	devicePortRaw, err := snmpSubtree(host, community, cdpCacheDevicePort)
	for k, v := range devicePortRaw {
		ids := strings.Split(k, ".")
		if len(ids) != 2 {
			slog.Error("unexpected snmp cdpCacheEntry id")
			continue
		}
		if entry, ok := cdpEntries[ids[0]]; ok {
			entry.DevicePort = fmt.Sprintf("%s", v)
		}
	}
	byInterface := make(map[string][]*cdpCacheEntry)
	for _, entry := range cdpEntries {
		if _, ok := byInterface[entry.InterfaceID]; ok {
			byInterface[entry.InterfaceID] = append(byInterface[entry.InterfaceID], entry)
		} else {
			byInterface[entry.InterfaceID] = []*cdpCacheEntry{entry}
		}
	}
	for iface, entry := range byInterface {
		j, err := json.Marshal(entry)
		if err != nil {
			return md, err
		}
		metadata.AddMeta("", opentsdb.TagSet{"host": host, "iface": iface}, "cdpCacheEntries", string(j), false)
	}
	if err != nil {
		return md, nil
	}
	return md, nil
}
