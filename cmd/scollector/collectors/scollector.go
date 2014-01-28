package collectors

import (
	"sync"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_scollector})
}

var (
	scollectorCounters = make(map[string]int)
	tlock              = sync.Mutex{}
)

func IncScollector(key string) {
	tlock.Lock()
	defer tlock.Unlock()
	scollectorCounters[key]++
}

func c_scollector() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	for k, v := range scollectorCounters {
		Add(&md, "scollector."+k, v, nil)
	}
	return md
}
