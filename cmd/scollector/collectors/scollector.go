package collectors

import (
	"runtime"
	"sync"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_scollector})
}

var (
	scollectorCounters = make(map[string]int64)
	tlock              = sync.Mutex{}
)

func IncScollector(key string, inc int) {
	tlock.Lock()
	defer tlock.Unlock()
	scollectorCounters[key] += int64(inc)
}

func c_scollector() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	tlock.Lock()
	defer tlock.Unlock()
	for k, v := range scollectorCounters {
		Add(&md, "scollector."+k, v, nil)
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	Add(&md, "scollector.alloc", ms.Alloc, nil)
	Add(&md, "scollector.goroutines", runtime.NumGoroutine(), nil)
	Add(&md, "scollector.numgc", ms.NumGC, nil)
	return md
}
