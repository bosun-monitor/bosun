package collectors

import (
	"fmt"
	"math/big"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

const sysUpTime = ".1.3.6.1.2.1.1.3.0" // "The time (in hundredths of a second) since the network management portion of the system was last re-initialized."

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
