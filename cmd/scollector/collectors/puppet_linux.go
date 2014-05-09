package collectors

import (
	"os"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_disabled_linux, init: puppetInit})
}

var (
	puppetEnable bool
	puppetLock   sync.Mutex
)

const (
	puppetPath     = "/var/lib/puppet/"
	puppetDisabled = "/var/lib/puppet/state/agent_disabled.lock"
)

func puppetEnabled() (b bool) {
	puppetLock.Lock()
	b = puppetEnable
	puppetLock.Unlock()
	return
}

func puppetInit() {
	update := func() {
		_, err := os.Stat(puppetPath)
		puppetLock.Lock()
		puppetEnable = err == nil
		puppetLock.Unlock()
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func puppet_disabled_linux() opentsdb.MultiDataPoint {
	if !puppetEnabled() {
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
