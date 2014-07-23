package collectors

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_opentsdb, init: tsdbInit})
}

const tsdbURL = "http://localhost:4242/api/stats"

var (
	tsdbEnable bool
	tsdbLock   sync.Mutex
)

func tsdbEnabled() (b bool) {
	tsdbLock.Lock()
	b = tsdbEnable
	tsdbLock.Unlock()
	return
}

func tsdbInit() {
	update := func() {
		resp, err := http.Get(tsdbURL)
		tsdbLock.Lock()
		defer tsdbLock.Unlock()
		if err != nil {
			tsdbEnable = false
			return
		}
		resp.Body.Close()
		tsdbEnable = resp.StatusCode == 200
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_opentsdb() opentsdb.MultiDataPoint {
	if !tsdbEnabled() {
		return nil
	}
	resp, err := http.Get(tsdbURL)
	if err != nil {
		slog.Error(err)
		return nil
	}
	defer resp.Body.Close()
	var md, tmp opentsdb.MultiDataPoint
	if err := json.NewDecoder(resp.Body).Decode(&tmp); err != nil {
		slog.Error(err)
		return nil
	}
	for _, v := range tmp {
		delete(v.Tags, "host")
		AddTS(&md, v.Metric, v.Timestamp, v.Value, v.Tags, metadata.Unknown, metadata.None, "")
	}
	return md
}
