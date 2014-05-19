// +build darwin linux

package collectors

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_hbase_region, init: hbrInit})
}

var (
	hbaseEnable bool
	hbaseLock   sync.Mutex
)

func hbaseEnabled() (b bool) {
	hbaseLock.Lock()
	b = hbaseEnable
	hbaseLock.Unlock()
	return
}

const hbrURL = "http://localhost:60030/jmx?qry=hadoop:service=RegionServer,name=RegionServerStatistics"

func hbrInit() {
	update := func() {
		resp, err := http.Get(hbrURL)
		hbaseLock.Lock()
		defer hbaseLock.Unlock()
		if err != nil {
			hbaseEnable = false
			return
		}
		resp.Body.Close()
		hbaseEnable = resp.StatusCode == 200
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_hbase_region() opentsdb.MultiDataPoint {
	if !hbaseEnabled() {
		return nil
	}
	var md opentsdb.MultiDataPoint
	res, err := http.Get(hbrURL)
	if err != nil {
		slog.Errorln(err)
		return nil
	}
	defer res.Body.Close()
	var r struct {
		Beans []map[string]interface{} `json:"beans"`
	}
	j := json.NewDecoder(res.Body)
	if err := j.Decode(&r); err != nil {
		slog.Errorln(err)
		return nil
	}
	if len(r.Beans) > 0 && len(r.Beans[0]) > 0 {
		for k, v := range r.Beans[0] {
			if _, ok := v.(float64); ok {
				Add(&md, "hbase.region."+k, v, nil)
			}
		}
	}
	return md
}
