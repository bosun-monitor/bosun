package collectors

import (
	"fmt"
	"io"
	"math/big"
	"time"

	"bosun.org/_third_party/github.com/mjibson/snmp"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var builtInSnmps = map[string]func(cfg conf.SNMP){"ifaces": SNMPIfaces, "cisco": SNMPCisco}

func SNMP(cfg conf.SNMP, mibs map[string]conf.MIB) error {
	if cfg.Host == "" {
		return fmt.Errorf("empty SNMP hostname")
	}
	if cfg.Community == "" {
		return fmt.Errorf("empty SNMP community")
	}
	if len(cfg.MIBs) == 0 {
		cfg.MIBs = []string{"ifaces", "cisco"}
	}
	for _, m := range cfg.MIBs {
		if f, ok := builtInSnmps[m]; ok {
			f(cfg)
		} else {
			mib, ok := mibs[m]
			if !ok {
				return fmt.Errorf("unknown MIB \"%s\" specified", m)
			}
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_snmp_generic(cfg, mib, m)
				},
				Interval: time.Second * 30,
				name:     fmt.Sprintf("snmp-generic-%s-%s", cfg.Host, m),
			})
		}
	}
	return nil
}

// snmp_subtree takes an oid and returns all data exactly one level below it. It
// produces an error if there is more than one level below.
func snmp_subtree(host, community, oid string) (map[int]interface{}, error) {
	rows, err := snmp.Walk(host, community, oid)
	if err != nil {
		return nil, err
	}
	m := make(map[int]interface{})
	for rows.Next() {
		switch oid {
		case ifHCInBroadcastPkts:
			a := new(big.Int)
			id, err := rows.Scan(&a)
			if err != nil {
				return nil, err
			}
			switch t := id.(type) {
			case int:
				m[t] = a
			default:
				return nil, fmt.Errorf("snmp subtree: only one level allowed")
			}
		default:
			var a interface{}
			id, err := rows.Scan(&a)
			if err != nil {
				return nil, err
			}
			switch t := id.(type) {
			case int:
				m[t] = a
			default:
				return nil, fmt.Errorf("snmp subtree: only one level allowed")
			}
		}
	}
	if err := rows.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return m, nil
}

func snmp_oid(host, community, oid string) (*big.Int, error) {
	v := new(big.Int)
	err := snmp.Get(host, community, oid, &v)
	return v, err
}

func c_snmp_generic(cfg conf.SNMP, mib conf.MIB, mibName string) (opentsdb.MultiDataPoint, error) {
	md := opentsdb.MultiDataPoint{}
	baseOid := mib.BaseOid

	for _, metric := range mib.Metrics {
		rate := metadata.RateType(metric.RateType)
		if rate == "" {
			rate = metadata.Gauge
		}
		unit := metadata.Unit(metric.Unit)
		if unit == "" {
			unit = metadata.None
		}
		v, err := snmp_oid(cfg.Host, cfg.Community, baseOid+metric.Oid)
		tagset := opentsdb.TagSet{"host": cfg.Host}
		if err == nil {
			Add(&md, metric.Metric, v, tagset, rate, unit, metric.Description)
		} else {
			return md, err
		}
	}
	for _, tree := range mib.Trees {
		treeOid := baseOid + tree.BaseOid
		names, err := snmp_subtree(cfg.Host, cfg.Community, treeOid+tree.LabelSourceOid)
		if err != nil {
			return md, err
		}
		tagset := opentsdb.TagSet{"host": cfg.Host}
		for _, metric := range tree.Metrics {
			rate := metadata.RateType(metric.RateType)
			if rate == "" {
				rate = metadata.Gauge
			}
			unit := metadata.Unit(metric.Unit)
			if unit == "" {
				unit = metadata.None
			}
			nodes, err := snmp_subtree(cfg.Host, cfg.Community, treeOid+metric.Oid)
			if err != nil {
				return md, err
			}
			if len(nodes) != len(names) {
				return md, fmt.Errorf("snmp tree for %s, and names have different lengths.", metric.Metric)
			}
			for i, v := range nodes {
				tagVal, ok := names[i]
				if !ok {
					return md, fmt.Errorf("Tree for tag %s has no entry for metric %s index %d.", tree.TagKey, metric.Metric, i)
				}
				tagset[tree.TagKey] = fmt.Sprintf("%s", tagVal)
				Add(&md, metric.Metric, v, tagset, rate, unit, metric.Description)
			}
		}
	}
	return md, nil
}
