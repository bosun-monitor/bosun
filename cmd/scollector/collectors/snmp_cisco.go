package collectors

import (
	"fmt"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/opentsdb"
)

// SNMPCisco registers a SNMP CISCO collector for the given community and host.
func SNMPCisco(cfg conf.SNMP) {

	mib := conf.MIB{}
	mib.BaseOid = "1.3.6.1.4.1.9.9"
	mib.Metrics = []conf.MIBMetric{
		conf.MIBMetric{
			Metric:      "cisco.cpu",
			Oid:         ".109.1.1.1.1.6",
			FallbackOid: ".109.1.1.1.1.6.1",
			Unit:        "percent",
		},
	}
	mib.Trees = []conf.MIBTree{
		conf.MIBTree{
			BaseOid:        ".48.1.1.1",
			TagKey:         "name",
			LabelSourceOid: ".2",
			Metrics: []conf.MIBMetric{
				conf.MIBMetric{
					Metric: "cisco.mem.used",
					Oid:    ".5",
				},
				conf.MIBMetric{
					Metric: "cisco.mem.free",
					Oid:    ".6",
				},
			},
		},
	}

	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_generic(cfg, mib)
		},
		Interval: time.Second * 30,
		name:     fmt.Sprintf("snmp-cisco-%s", cfg.Host),
	})
}
