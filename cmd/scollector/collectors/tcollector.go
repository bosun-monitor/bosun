package collectors

import (
	"sync"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, Collector{F: c_tcollector})
}

var (
	tcollectorCounters = make(map[string]int)
	tlock              = sync.Mutex{}
)

func IncTcollector(key string) {
	tlock.Lock()
	defer tlock.Unlock()
	tcollectorCounters[key]++
}

func c_tcollector() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	for k, v := range tcollectorCounters {
		Add(&md, "tcollector."+k, v, nil)
	}
	return md
}
