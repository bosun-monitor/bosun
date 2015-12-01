package collectors

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"bosun.org/_third_party/github.com/mjibson/snmp"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var builtInSNMPs = map[string]func(cfg conf.SNMP){
	"ifaces":   SNMPIfaces,
	"cisco":    SNMPCisco,
	"bridge":   SNMPBridge,
	"ips":      SNMPIPAddresses,
	"ciscobgp": SNMPCiscoBGP,
	"sys":      SNMPSys,
}

func SNMP(cfg conf.SNMP, mibs map[string]conf.MIB) error {
	if cfg.Host == "" {
		return fmt.Errorf("empty SNMP hostname")
	}
	if cfg.Community == "" {
		return fmt.Errorf("empty SNMP community")
	}
	if len(cfg.MIBs) == 0 {
		cfg.MIBs = []string{"ifaces", "cisco", "bridge"}
	}
	for _, m := range cfg.MIBs {
		mib, ok := mibs[m]
		if ok {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return GenericSnmp(cfg, mib)
				},
				name: fmt.Sprintf("snmp-generic-%s-%s", cfg.Host, m),
			})
		} else {
			if f, ok := builtInSNMPs[m]; ok {
				f(cfg)
			} else {
				return fmt.Errorf("unknown MIB \"%s\" specified", m)
			}
		}
	}
	return nil
}

// snmp_subtree takes an oid and returns all data exactly one level below it. It
// produces an error if there is more than one level below.
func snmp_subtree(host, community, oid string) (map[string]interface{}, error) {
	rows, err := snmp.Walk(host, community, oid)
	if err != nil {
		return nil, err
	}
	m := make(map[string]interface{})
	for rows.Next() {
		key := ""
		var a interface{}
		switch oid {
		case ifHCInBroadcastPkts:
			a = new(big.Int)
			id, err := rows.Scan(&a)
			if err != nil {
				return nil, err
			}
			switch t := id.(type) {
			case int:
				key = fmt.Sprint(t)
			default:
				return nil, fmt.Errorf("snmp subtree: only one level allowed")
			}
		default:
			id, err := rows.Scan(&a)
			if err != nil {
				return nil, err
			}
			switch t := id.(type) {
			case int:
				key = fmt.Sprint(t)
			case []int:
				key = snmpOidArrayToString(t)
			default:
				return nil, fmt.Errorf("Unknown key type: %s", reflect.TypeOf(id).String())
			}
		}
		m[key] = a
	}
	if err := rows.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return m, nil
}

// recombine an oid array to a dot-delimited string
func snmpOidArrayToString(path []int) string {
	s := make([]string, len(path))
	for i := range path {
		s[i] = fmt.Sprint(path[i])
	}
	return strings.Join(s, ".")
}

func snmp_oid(host, community, oid string) (*big.Int, error) {
	v := new(big.Int)
	err := snmp.Get(host, community, oid, &v)
	return v, err
}

func snmp_convertToFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float64:
		return val, nil
	case *big.Int:
		return float64(val.Int64()), nil
	case string:
		return strconv.ParseFloat(val, 64)
	case []uint8:
		return strconv.ParseFloat(string(val), 64)
	default:
		return 0, fmt.Errorf("Cannot convert type %s to float64", reflect.TypeOf(v))
	}
}

func combineOids(oid, base string) string {
	if oid != "" && oid[0] == '.' {
		return base + oid
	}
	return oid
}

func GenericSnmp(cfg conf.SNMP, mib conf.MIB) (opentsdb.MultiDataPoint, error) {
	md := opentsdb.MultiDataPoint{}
	baseOid := mib.BaseOid

	rateUnitTags := func(m conf.MIBMetric) (r metadata.RateType, u metadata.Unit, t opentsdb.TagSet, err error) {
		if r = metadata.RateType(m.RateType); r == "" {
			r = metadata.Gauge
		}
		if u = metadata.Unit(m.Unit); u == "" {
			u = metadata.None
		}
		if m.Tags == "" {
			t = make(opentsdb.TagSet)
		} else {
			t, err = opentsdb.ParseTags(m.Tags)
			if err != nil {
				return "", "", nil, err
			}
		}
		t["host"] = cfg.Host
		return
	}

	for _, metric := range mib.Metrics {
		rate, unit, tagset, err := rateUnitTags(metric)
		if err != nil {
			return md, err
		}

		v, err := snmp_oid(cfg.Host, cfg.Community, combineOids(metric.Oid, baseOid))
		if err != nil && metric.FallbackOid != "" {
			v, err = snmp_oid(cfg.Host, cfg.Community, combineOids(metric.FallbackOid, baseOid))
		}
		if err != nil {
			return md, err
		}
		val, err := snmp_convertToFloat(v)
		if err != nil {
			return md, err
		}
		if metric.Scale != 0 {
			val = val * metric.Scale
		}
		Add(&md, metric.Metric, val, tagset, rate, unit, metric.Description)
	}

	for _, tree := range mib.Trees {
		treeOid := combineOids(tree.BaseOid, baseOid)
		tagCache := make(map[string]map[string]interface{}) // tag key to map of values
		for _, tag := range tree.Tags {
			if tag.Oid == "idx" {
				continue
			}
			vals, err := snmp_subtree(cfg.Host, cfg.Community, combineOids(tag.Oid, treeOid))
			if err != nil {
				return md, err
			}
			tagCache[tag.Key] = vals
		}
		for _, metric := range tree.Metrics {
			rate, unit, tagset, err := rateUnitTags(metric)
			if err != nil {
				return md, err

			}
			nodes, err := snmp_subtree(cfg.Host, cfg.Community, combineOids(metric.Oid, treeOid))
			if err != nil && metric.FallbackOid != "" {
				nodes, err = snmp_subtree(cfg.Host, cfg.Community, combineOids(metric.FallbackOid, treeOid))
			}
			if err != nil {
				return md, err
			}
			// check all lengths
			for k, list := range tagCache {
				if len(list) != len(nodes) {
					return md, fmt.Errorf("snmp tree for tag key %s, and metric %s do not have same length", k, metric.Metric)
				}
			}
			for i, v := range nodes {
				for _, tag := range tree.Tags {
					var tagVal interface{}
					if tag.Oid == "idx" {
						tagVal = i
					} else {
						var ok bool
						tagVal, ok = tagCache[tag.Key][i]
						if !ok {
							return md, fmt.Errorf("tree for tag %s has no entry for metric %s index %s", tag.Key, metric.Metric, i)
						}
					}
					if byteSlice, ok := tagVal.([]byte); ok {
						tagVal = string(byteSlice)
					}
					tagset[tag.Key] = fmt.Sprint(tagVal)
				}
				val, err := snmp_convertToFloat(v)
				if err != nil {
					return md, err
				}
				if metric.Scale != 0 {
					val = val * metric.Scale
				}
				Add(&md, metric.Metric, val, tagset, rate, unit, metric.Description)
			}
		}
	}
	return md, nil
}
