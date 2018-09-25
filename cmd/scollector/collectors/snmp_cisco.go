package collectors

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

//SNMPCiscoASA registers a SNMP CISCO IOS collector for the given community and host.
func SNMPCiscoASA(cfg conf.SNMP) {
	cpuIntegrator := getTsIntegrator()
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				// Currently the trees are the same between IOS and NXOS
				// But registering it this way will make it so future changes
				// won't require a configuration change
				return c_cisco_ios(cfg.Host, cfg.Community, cpuIntegrator)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-asa-%s", cfg.Host),
		},
		//
		// Execute ASA-specific checks in c_cisco_asa
		//
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_asa(cfg.Host, cfg.Community)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-asa-specific-%s", cfg.Host),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_desc(cfg.Host, cfg.Community)
			},
			Interval: time.Minute * 5,
			name:     fmt.Sprintf("snmp-cisco-desc-%s", cfg.Host),
		},
	)
}

//SNMPCiscoIOS registers a SNMP CISCO IOS collector for the given community and host.
func SNMPCiscoIOS(cfg conf.SNMP) {
	cpuIntegrator := getTsIntegrator()
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_ios(cfg.Host, cfg.Community, cpuIntegrator)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-ios-%s", cfg.Host),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_desc(cfg.Host, cfg.Community)
			},
			Interval: time.Minute * 5,
			name:     fmt.Sprintf("snmp-cisco-desc-%s", cfg.Host),
		},
	)
}

//SNMPCiscoNXOS registers a SNMP Cisco's NXOS collector (i.e. nexus switches) for the given community and host.
func SNMPCiscoNXOS(cfg conf.SNMP) {
	cpuIntegrator := getTsIntegrator()
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_nxos(cfg.Host, cfg.Community, cpuIntegrator)
			},
			Interval: time.Second * 30,
			name:     fmt.Sprintf("snmp-cisco-nxos-%s", cfg.Host),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cisco_desc(cfg.Host, cfg.Community)
			},
			Interval: time.Minute * 5,
			name:     fmt.Sprintf("snmp-cisco-desc-%s", cfg.Host),
		},
	)
}

const (
	ciscoBaseOID         = "1.3.6.1.4.1.9.9"
	cpmCPUTotal5secRev   = ".109.1.1.1.1.6"
	asaConnInUseCurrent  = ".147.1.2.2.2.1.5.40.6"
	asaConnInUseMax      = ".147.1.2.2.2.1.5.40.7"
	ciscoMemoryPoolTable = ".48.1.1.1"
)

const (
	ciscoMemoryPoolFreeDesc = "The number of bytes from the memory pool that are currently in use by applications on the managed device."
	ciscoMemoryPoolUsedDesc = "the number of bytes from the memory pool that are currently unused on the managed device."
	asaConnInUseCurrentDesc = "The number of connections currently registered in the ASA firewall."
	asaConnInUseMaxDesc     = "The maximum number of connections to an ASA firewall since last power cycle."
)

type ciscoMemoryPoolEntry struct {
	PoolType string
	Used     int64
	Free     int64
}

func ciscoASAConn(host, community string, ts opentsdb.TagSet, md *opentsdb.MultiDataPoint) error {
	connCurrent, err := snmp_oid(host, community, ciscoBaseOID+asaConnInUseCurrent)
	if err != nil {
		return fmt.Errorf("Error when receiving ASA current connection count.")
	}

	connMax, err := snmp_oid(host, community, ciscoBaseOID+asaConnInUseMax)
	if err != nil {
		return fmt.Errorf("Error when receiving ASA Max connections count.")
	}

	Add(md, "cisco.asa.conn_current", connCurrent, ts, metadata.Gauge, metadata.Connection, asaConnInUseCurrentDesc)
	Add(md, "cisco.asa.conn_max", connMax, ts, metadata.Gauge, metadata.Connection, asaConnInUseMaxDesc)
	return nil

}

func ciscoCPU(host, community string, ts opentsdb.TagSet, cpuIntegrator tsIntegrator, md *opentsdb.MultiDataPoint) error {
	cpuRaw, err := snmp_subtree(host, community, ciscoBaseOID+cpmCPUTotal5secRev)
	if err != nil {
		return err
	}

	cpu := make(map[string]int)
	for k, v := range cpuRaw {
		pct, err := strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil {
			return err
		}
		cpu[k] = pct
	}
	if len(cpu) > 1 {
		return fmt.Errorf("expected only one cpu when monitoring cisco cpu via cpmCPUTotal5secRev")
	}
	for _, pct := range cpu {
		Add(md, "cisco.cpu", pct, ts, metadata.Gauge, metadata.Pct, "")
		Add(md, osCPU, cpuIntegrator(time.Now().Unix(), float64(pct)), ts, metadata.Counter, metadata.Pct, "")
	}
	return nil
}

func c_cisco_asa(host, community string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	ts := opentsdb.TagSet{"host": host}

	// ASA connection counts
	if err := ciscoASAConn(host, community, ts, &md); err != nil {
		return md, err
	}
	return md, nil
}

func c_cisco_ios(host, community string, cpuIntegrator tsIntegrator) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	ts := opentsdb.TagSet{"host": host}
	// CPU
	if err := ciscoCPU(host, community, ts, cpuIntegrator, &md); err != nil {
		return md, err
	}
	// ÎMemory
	memRaw, err := snmp_subtree(host, community, ciscoBaseOID+ciscoMemoryPoolTable)
	if err != nil {
		return md, fmt.Errorf("failed to get ciscoMemoryPoolTable for host %v: %v", host, err)
	}
	idToPoolEntry := make(map[string]*ciscoMemoryPoolEntry)
	for id, value := range memRaw {
		sp := strings.SplitN(id, ".", 2)
		if len(sp) != 2 {
			slog.Errorf("expected length of 2 for snmp sub OID (%v) for ciscoMemoryPoolTable for host %v: got length %v", id, host, len(sp))
		}
		columnID := sp[0]
		entryID := sp[1]
		if _, ok := idToPoolEntry[entryID]; !ok {
			idToPoolEntry[entryID] = &ciscoMemoryPoolEntry{}
		}
		switch columnID {
		case "2":
			if v, ok := value.([]byte); ok {
				if m, ok := idToPoolEntry[entryID]; ok {
					m.PoolType = string(v)
				} else {
					slog.Errorf("failed to find cisco memory pool entry for entry id %v on host %v for memory pool type", entryID, host)
				}
			} else {
				slog.Errorf("failed to convert memory pool label %v to []byte for host %v", value, host)
			}
		case "5":
			if v, ok := value.(int64); ok {
				if m, ok := idToPoolEntry[entryID]; ok {
					m.Used = v
				} else {
					slog.Errorf("failed to find cisco memory pool entry for entry id %v on host %v for used memory", entryID, host)
				}
			} else {
				slog.Errorf("failed to convert used memory value %v to int64 for host %v", value, host)
			}
		case "6":
			if v, ok := value.(int64); ok {
				if m, ok := idToPoolEntry[entryID]; ok {
					m.Free = v
				} else {
					slog.Errorf("failed to find cisco memory pool entry for entry id %v on host %v for free memory", entryID, host)
				}
			} else {
				slog.Errorf("failed to convert used memory value %v to int64 for host %v", value, host)
			}
		}
	}
	var totalFreeMem int64
	var totalUsedMem int64
	for _, entry := range idToPoolEntry {
		ts := ts.Copy().Merge(opentsdb.TagSet{"name": entry.PoolType})
		Add(&md, "cisco.mem.used", entry.Used, ts, metadata.Gauge, metadata.Bytes, ciscoMemoryPoolUsedDesc)
		Add(&md, "cisco.mem.free", entry.Free, ts, metadata.Gauge, metadata.Bytes, ciscoMemoryPoolFreeDesc)
		totalFreeMem += entry.Free
		totalUsedMem += entry.Used
	}
	Add(&md, osMemFree, totalFreeMem, ts, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
	Add(&md, osMemUsed, totalUsedMem, ts, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
	totalMem := totalFreeMem + totalUsedMem
	Add(&md, osMemTotal, totalMem, ts, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
	Add(&md, osMemPctFree, int64(float64(totalFreeMem)/float64(totalMem)*100), ts, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
	return md, nil
}

const (
	cpmCPUTotalEntry = ".109.1.1.1.1"
)

func c_cisco_nxos(host, community string, cpuIntegrator tsIntegrator) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	ts := opentsdb.TagSet{"host": host}
	// CPU
	if err := ciscoCPU(host, community, ts, cpuIntegrator, &md); err != nil {
		return md, err
	}
	// Memory
	memRaw, err := snmp_subtree(host, community, ciscoBaseOID+cpmCPUTotalEntry)
	if err != nil {
		return md, fmt.Errorf("failed to get cpmCPUTotalEntry (for memory) for host %v: %v", host, err)
	}
	var usedMem, freeMem, totalMem int64
	var usedOk, freeOk bool
	for id, value := range memRaw {
		var v int64
		switch id {
		case "12.1":
			if v, usedOk = value.(int64); usedOk {
				usedMem = v * 2 << 9 // KiB to Bytes
				totalMem += usedMem
				Add(&md, osMemUsed, usedMem, ts, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
			} else {
				slog.Errorf("failed to convert used memory %v to int64 for host %v", value, host)
			}
		case "13.1":
			if v, freeOk = value.(int64); freeOk {
				freeMem = v * 2 << 9
				totalMem += freeMem
				Add(&md, osMemFree, freeMem, ts, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
			} else {
				slog.Errorf("failed to convert free memory %v to int64 for host %v", value, host)
			}
		}
	}
	if usedOk && freeOk {
		Add(&md, osMemTotal, totalMem, ts, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
		Add(&md, osMemPctFree, int64(float64(freeMem)/float64(totalMem)*100), ts, metadata.Gauge, metadata.Pct, osMemPctFreeDesc)
	} else {
		slog.Errorf("failed to get both free and used memory for host %v", host)
	}
	return md, nil
}

func c_cisco_desc(host, community string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	desc, err := getSNMPDesc(host, community)
	if err != nil {
		return md, err
	}
	if desc == "" {
		return md, fmt.Errorf("empty description string (used to get OS version) for cisco host %v", host)
	}
	metadata.AddMeta("", opentsdb.TagSet{"host": host}, "versionCaption", desc, false)
	return md, nil
}
