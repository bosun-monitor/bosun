package collectors

import (
	"os"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_disabled_func_linux})
}

func puppet_disabled_func_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	disabled := 0
	filename := "/var/lib/puppet/state/agent_disabled.lock"
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		disabled = 1
	}
	Add(&md, "puppet.disabled", disabled, nil)
	return md
}
