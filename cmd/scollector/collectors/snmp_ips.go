package collectors

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

const ifIPAdEntAddr = ".1.3.6.1.2.1.4.20.1"

// SNMPIfaces registers a SNMP Interfaces collector for the given community and host.
func SNMPIPAddresses(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_ips(cfg.Community, cfg.Host)
		},
		Interval: time.Minute * 1,
		name:     fmt.Sprintf("snmp-ips-%s", cfg.Host),
	})
}

type ipAdEntAddr struct {
	InterfaceId int64
	net.IPNet
}

func c_snmp_ips(community, host string) (opentsdb.MultiDataPoint, error) {
	ifIPAdEntAddrRaw, err := snmp_subtree(host, community, ifIPAdEntAddr)
	if err != nil {
		return nil, err
	}
	ipAdEnts := make(map[string]*ipAdEntAddr)
	for id, value := range ifIPAdEntAddrRaw {
		// Split entry type id from ip address
		sp := strings.SplitN(id, ".", 2)
		if len(sp) != 2 {
			slog.Errorln("unexpected length of snmp resonse")
		}
		typeId := sp[0]
		address := sp[1]
		if _, ok := ipAdEnts[address]; !ok {
			ipAdEnts[address] = &ipAdEntAddr{}
		}
		switch typeId {
		case "1":
			if v, ok := value.([]byte); ok {
				ipAdEnts[address].IP = v
			}
		case "2":
			if v, ok := value.(int64); ok {
				ipAdEnts[address].InterfaceId = v
			}
		case "3":
			if v, ok := value.([]byte); ok {
				ipAdEnts[address].Mask = v
			}
		}
	}
	ipsByInt := make(map[int64][]net.IPNet)
	for _, ipNet := range ipAdEnts {
		ipsByInt[ipNet.InterfaceId] = append(ipsByInt[ipNet.InterfaceId], ipNet.IPNet)
	}
	for intId, ipNets := range ipsByInt {
		var ips []string
		for _, ipNet := range ipNets {
			ips = append(ips, ipNet.String())
		}
		j, err := json.Marshal(ips)
		if err != nil {
			slog.Errorf("error marshaling ips for host %v: %v", host, err)
		}
		metadata.AddMeta("", opentsdb.TagSet{"host": host, "iface": fmt.Sprintf("%v", intId)}, "addresses", string(j), false)
	}
	return nil, nil
}
