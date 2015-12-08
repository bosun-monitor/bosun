package collectors

import (
	"fmt"
	"strconv"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	fortinetBaseOID        = ".1.3.6.1.4.1.12356.101"
	fortinetCPU            = ".4.4.2.1.3"
	fortinetMemTotal       = ".4.1.5.0"
	fortinetMemPercentUsed = ".4.1.4.0"
)

// SNMPFortinet registers a SNMP Fortinet collector for the given community and host.
func SNMPFortinet(cfg conf.SNMP) {
	cpuIntegrators := make(map[string]tsIntegrator)
	mib := conf.MIB{
		BaseOid: "1.3.6.1.4.1.12356.101",
		Metrics: []conf.MIBMetric{
			{Metric: "fortinet.disk.used", Oid: ".4.1.6.0", Unit: "MiB", RateType: "gauge", Description: "Disk space used", FallbackOid: "", Tags: "", Scale: 0},
			{Metric: "fortinet.disk.total", Oid: ".4.1.7.0", Unit: "MiB", RateType: "gauge", Description: "Disk space total", FallbackOid: "", Tags: "", Scale: 0},
			{Metric: "fortinet.session.count", Oid: ".11.2.2.1.1.1", Unit: "sessions", RateType: "gauge", Description: "Total number of current sessions being tracked", FallbackOid: "", Tags: "", Scale: 0},
			{Metric: "fortinet.vpn.tunnel_up_count", Oid: ".12.1.1.0", Unit: "tunnel count", RateType: "gauge", Description: "Total number of up VPN tunnels", FallbackOid: "", Tags: "", Scale: 0},
		},
		Trees: []conf.MIBTree{
			{
				BaseOid: ".4.3.2.1",
				Tags:    []conf.MIBTag{{Key: "name", Oid: ".2"}},
				Metrics: []conf.MIBMetric{
					{Metric: "fortinet.hardware.sensor.value", Oid: ".3", Unit: "", RateType: "gauge", Description: "Fortinet hardware sensor values (units vary)", FallbackOid: "", Tags: "", Scale: 0},
					{Metric: "fortinet.hardware.sensor.alarm", Oid: ".4", Unit: "", RateType: "gauge", Description: "Fortinet hardware sensor alarm state (1=alarm)", FallbackOid: "", Tags: "", Scale: 0},
				},
			},
			{
				BaseOid: ".13.2.1.1",
				Tags:    []conf.MIBTag{{Key: "name", Oid: ".11"}},
				Metrics: []conf.MIBMetric{
					{Metric: "fortinet.ha.sync_state", Oid: ".12", Unit: "", RateType: "gauge", Description: "Fortinet HA state (0 = unsynced)", FallbackOid: "", Tags: "", Scale: 0},
				}},
			{
				BaseOid: ".12.2.2.1", Tags: []conf.MIBTag{{Key: "name", Oid: ".2"}},
				Metrics: []conf.MIBMetric{
					{Metric: "fortinet.vpn.state", Oid: ".20", Unit: "", RateType: "gauge", Description: "VPN tunnel state (1=down, 2=up)", FallbackOid: "", Tags: "", Scale: 0},
				},
			},
		},
	}
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return GenericSnmp(cfg, mib)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-fortinet-%s", cfg.Host),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_fortinet_os(cfg.Host, cfg.Community, cpuIntegrators)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-fortinet-os-%s", cfg.Host),
		},
	)
}

func c_fortinet_os(host, community string, cpuIntegrators map[string]tsIntegrator) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	ts := opentsdb.TagSet{"host": host}
	// CPU
	cpuRaw, err := snmp_subtree(host, community, fortinetBaseOID+fortinetCPU)
	if err != nil {
		return md, err
	}
	coreCount := len(cpuRaw)
	var totalPercent int
	for id, v := range cpuRaw {
		cpuVal, err := strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil {
			return md, fmt.Errorf("couldn't convert cpu value to int for fortinet cpu utilization on host %v: %v", host, err)
		}
		ts := ts.Copy().Merge(opentsdb.TagSet{"processor": id})
		Add(&md, "fortinet.cpu.percent_used", cpuVal, ts, metadata.Gauge, metadata.Pct, "")
		totalPercent += cpuVal
	}
	if _, ok := cpuIntegrators[host]; !ok {
		cpuIntegrators[host] = getTsIntegrator()
	}
	Add(&md, osCPU, cpuIntegrators[host](time.Now().Unix(), float64(totalPercent)/float64(coreCount)), opentsdb.TagSet{"host": host}, metadata.Gauge, metadata.Pct, "")

	// Memory
	memTotal, err := snmp_oid(host, community, fortinetBaseOID+fortinetMemTotal)
	if err != nil {
		return md, fmt.Errorf("failed to get total memory for fortinet host %v: %v", host, err)
	}
	memTotalBytes := memTotal.Int64() * 2 << 9 // KiB to Bytes
	Add(&md, "fortinet.mem.total", memTotal, ts, metadata.Gauge, metadata.KBytes, "The total memory in kilobytes.")
	Add(&md, osMemTotal, memTotalBytes, ts, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
	memPctUsed, err := snmp_oid(host, community, fortinetBaseOID+fortinetMemPercentUsed)
	if err != nil {
		return md, fmt.Errorf("failed to get percent of memory used for fortinet host %v: %v", host, err)
	}
	Add(&md, "fortinet.mem.percent_used", memPctUsed, ts, metadata.Gauge, metadata.Pct, "The percent of memory used.")
	memPctUsedFloat := float64(memPctUsed.Int64()) / 100
	memPctFree := 100 - memPctUsed.Int64()
	Add(&md, osMemPctFree, memPctFree, ts, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
	memFree := float64(memTotalBytes) * (float64(1) - memPctUsedFloat)
	Add(&md, osMemFree, int64(memFree), ts, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
	Add(&md, osMemUsed, int64(float64(memTotalBytes)-memFree), ts, metadata.Gauge, metadata.Bytes, osMemUsedDesc)

	return md, nil
}
