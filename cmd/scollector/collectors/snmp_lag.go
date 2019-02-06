package collectors

import (
	"fmt"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	dot3adAggPortAttachedAggID = ".1.2.840.10006.300.43.1.2.1.1.13"
)

// SNMPLag registers a SNMP Interfaces collector for the given community and host.
func SNMPLag(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_lag(cfg.Community, cfg.Host)
		},
		Interval:      time.Second * 30,
		CollectorName: fmt.Sprintf("snmp-lag-%s", cfg.Host),
	})
}

func c_snmp_lag(community, host string) (opentsdb.MultiDataPoint, error) {
	ifNamesRaw, err := snmp_subtree(host, community, dot3adAggPortAttachedAggID)
	if err != nil {
		return nil, err
	}
	for k, v := range ifNamesRaw {
		tags := opentsdb.TagSet{"host": host, "iface": k}
		metadata.AddMeta("", tags, "masterIface", fmt.Sprintf("%v", v), false)
	}
	return nil, nil
}
