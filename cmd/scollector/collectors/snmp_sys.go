package collectors

import (
	"fmt"
	"math/big"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const (
	sysUpTime = ".1.3.6.1.2.1.1.3.0" // "The time (in hundredths of a second) since the network management portion of the system was last re-initialized."
	sysDescr  = ".1.3.6.1.2.1.1.1.0"
)

// SNMPSys registers a SNMP system data collector for the given community and host.
func SNMPSys(cfg conf.SNMP) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_snmp_sys(cfg.Host, cfg.Community)
		},
		Interval: time.Minute * 1,
		name:     fmt.Sprintf("snmp-sys-%s", cfg.Host),
	})
}

func c_snmp_sys(host, community string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	uptime, err := snmp_oid(host, community, sysUpTime)
	if err != nil {
		return md, err
	}
	Add(&md, osSystemUptime, uptime.Int64()/big.NewInt(100).Int64(), opentsdb.TagSet{"host": host}, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
	return md, nil
}

// Description may mean different things so it isn't called in sys, for example
// with cisco it is the os version
func getSNMPDesc(host, community string) (description string, err error) {
	description, err = snmpOidString(host, community, sysDescr)
	if err != nil {
		return description, fmt.Errorf("failed to fetch description for host %v: %v", host, err)
	}
	return
}
