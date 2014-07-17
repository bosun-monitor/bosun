// +build darwin linux

package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_hbase_region, init: hbrInit})
	collectors = append(collectors, &IntervalCollector{F: c_hbase_replication, init: hbrepInit})
}

type enabled struct {
	Enable bool
	Lock   sync.Mutex
}

var (
	hbrEnable   enabled
	hbrepEnable enabled
)

func (e enabled) Enabled() (b bool) {
	e.Lock.Lock()
	b = e.Enable
	e.Lock.Unlock()
	return
}

const hbrURL = "http://localhost:60030/jmx?qry=hadoop:service=RegionServer,name=RegionServerStatistics"
const hbrepURL = "http://localhost:60030/jmx?qry=hadoop:service=Replication,name=*"

func testUrl(url string, e *enabled) func() {
	update := func() {
		resp, err := http.Get(url)
		e.Lock.Lock()
		defer e.Lock.Unlock()
		if err != nil {
			e.Enable = false
			return
		}
		resp.Body.Close()
		e.Enable = resp.StatusCode == 200
	}
	return update
}

func hbrInit() {
	update := testUrl(hbrURL, &hbrEnable)
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_hbase_region() opentsdb.MultiDataPoint {
	if !hbrEnable.Enabled() {
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

func hbrepInit() {
	update := testUrl(hbrepURL, &hbrepEnable)
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_hbase_replication() opentsdb.MultiDataPoint {
	if !hbrepEnable.Enabled() {
		return nil
	}
	var md opentsdb.MultiDataPoint
	res, err := http.Get(hbrepURL)
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
	for i, section := range r.Beans {
		var instance string
		for k, v := range section {
			if k == "name" && strings.HasPrefix(v.(string), "hadoop:service=Replication,name=ReplicationSource for") {
				s := strings.Split(v.(string), " ")
				fmt.Println(s[len(s)-1])
				instance = s[len(s)-1]
				continue
			}
		}
		for k, v := range r.Beans[i] {
			if _, ok := v.(float64); ok {
				if instance == "" {
					Add(&md, "hbase.replication."+k, v, nil)
					continue
				}
				Add(&md, "hbase.replication."+k, v, opentsdb.TagSet{"instance": instance})
			}
		}
		instance = ""
	}
	return md
}
