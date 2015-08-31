package collectors

import (
	"fmt"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// SNMPCisco registers a SNMP CISCO collector for the given community and host.
func SNMPCisco(cfg conf.SNMP) {
	mib := conf.MIB{
		BaseOid: "1.3.6.1.4.1.9.9",
		Metrics: []conf.MIBMetric{
			{
				Metric:      "cisco.cpu",
				Oid:         ".109.1.1.1.1.6",
				FallbackOid: ".109.1.1.1.1.6.1",
				Unit:        metadata.Pct,
			},
		},
		Trees: []conf.MIBTree{
			{
				BaseOid: ".48.1.1.1",
				Tags: []conf.MIBTag{
					{Key: "name", Oid: ".2"},
				},
				Metrics: []conf.MIBMetric{
					{
						Metric: "cisco.mem.used",
						Oid:    ".5",
					},
					{
						Metric: "cisco.mem.free",
						Oid:    ".6",
					},
				},
			},
		},
	}

	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return GenericSnmp(cfg, mib)
		},
		Interval: time.Second * 30,
		name:     fmt.Sprintf("snmp-cisco-%s", cfg.Host),
	})
}
