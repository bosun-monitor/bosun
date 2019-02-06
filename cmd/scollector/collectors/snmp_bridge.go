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

// SNMP Bridge registers
func SNMPBridge(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_bridge(cfg.Community, cfg.Host)
		},
		Interval:      time.Minute * 5,
		CollectorName: fmt.Sprintf("snmp-bridge-%s", cfg.Host),
	})
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_cdp(cfg.Community, cfg.Host)
		},
		Interval:      time.Minute * 5,
		CollectorName: fmt.Sprintf("snmp-cdp-%s", cfg.Host),
	})
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
	cdpCacheDeviceId   = "1.3.6.1.4.1.9.9.23.1.2.1.1.6"
	cdpCacheDevicePort = "1.3.6.1.4.1.9.9.23.1.2.1.1.7"
)

type cdpCacheEntry struct {
	InterfaceId string `json:"-"`
	DeviceId    string
	DevicePort  string
}

func c_snmp_cdp(community, host string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	cdpEntries := make(map[string]*cdpCacheEntry)
	deviceIdRaw, err := snmp_subtree(host, community, cdpCacheDeviceId)
	if err != nil {
		return md, err
	}
	for k, v := range deviceIdRaw {
		ids := strings.Split(k, ".")
		if len(ids) != 2 {
			slog.Error("unexpected snmp cdpCacheEntry id")
			continue
		}
		cdpEntries[ids[0]] = &cdpCacheEntry{}
		cdpEntries[ids[0]].DeviceId = fmt.Sprintf("%s", v)
		cdpEntries[ids[0]].InterfaceId = ids[1]
	}
	devicePortRaw, err := snmp_subtree(host, community, cdpCacheDevicePort)
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
		if _, ok := byInterface[entry.InterfaceId]; ok {
			byInterface[entry.InterfaceId] = append(byInterface[entry.InterfaceId], entry)
		} else {
			byInterface[entry.InterfaceId] = []*cdpCacheEntry{entry}
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
