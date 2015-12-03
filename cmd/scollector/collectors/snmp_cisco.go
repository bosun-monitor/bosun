package collectors

import (
	"fmt"
	"strconv"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

// SNMPCisco registers a SNMP CISCO collector for the given community and host.
func SNMPCisco(cfg conf.SNMP) {
	mib := conf.MIB{
		BaseOid: "1.3.6.1.4.1.9.9",
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
	cpuIntegrator := getTsIntegrator()
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return GenericSnmp(cfg, mib)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-%s", cfg.Host),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_cpu(cfg.Host, cfg.Community, cpuIntegrator)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-cpu-%s", cfg.Host),
		},
	)
}

const (
	cpmCPUTotal5secRev = ".1.3.6.1.4.1.9.9.109.1.1.1.1.6"
)

func c_cisco_cpu(host, community string, cpuIntegrator tsIntegrator) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	cpuRaw, err := snmp_subtree(host, community, cpmCPUTotal5secRev)
	if err != nil {
		return md, err
	}
	tags := opentsdb.TagSet{"host": host}
	cpu := make(map[string]int)
	for k, v := range cpuRaw {
		pct, err := strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil {
			return md, err
		}
		cpu[k] = pct
	}
	if len(cpu) > 1 {
		return md, fmt.Errorf("expected only one cpu when monitoring cisco cpu via cpmCPUTotal5secRev")
	}
	for _, pct := range cpu {
		Add(&md, "cisco.cpu", pct, tags, metadata.Gauge, metadata.Pct, "")
		Add(&md, osCPU, cpuIntegrator(time.Now().Unix(), float64(pct)), tags, metadata.Counter, metadata.Pct, "")
	}
	return md, nil
}
