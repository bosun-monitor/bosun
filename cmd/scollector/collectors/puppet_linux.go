package collectors

import (
	"os"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_disabled_func_linux, init: puppetInit})
}

var puppetExists bool

const puppetPath = "/var/lib/puppet/"
const puppetDisabled = "/var/lib/puppet/state/agent_disabled.lock"

func puppetInit() {
	update := func() {
		if _, err := os.Stat(puppetPath); err == nil {
			puppetExists = true
		}
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func puppet_disabled_func_linux() opentsdb.MultiDataPoint {
	if !puppetExists {
		return nil
	}
	var md opentsdb.MultiDataPoint
	disabled := 0
	if _, err := os.Stat(puppetDisabled); !os.IsNotExist(err) {
		disabled = 1
	}
	Add(&md, "puppet.disabled", disabled, nil)
	return md
}
