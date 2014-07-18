// +build darwin linux

package collectors

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_hbase_region, init: hbrInit})
	collectors = append(collectors, &IntervalCollector{F: c_hbase_replication, init: hbrepInit})
}

type hEnabled struct {
	Enable bool
	sync.Mutex
}

var (
	hbrEnable   hEnabled
	hbrepEnable hEnabled
)

func (e hEnabled) Enabled() (b bool) {
	e.Lock()
	b = e.Enable
	e.Unlock()
	return
}

const hbrURL = "http://localhost:60030/jmx?qry=hadoop:service=RegionServer,name=RegionServerStatistics"
const hbrepURL = "http://localhost:60030/jmx?qry=hadoop:service=Replication,name=*"

func hTestUrl(url string, e *hEnabled) func() {
	update := func() {
		resp, err := http.Get(url)
		e.Lock()
		defer e.Unlock()
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
	update := hTestUrl(hbrURL, &hbrEnable)
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

type jmx struct {
	Beans []map[string]interface{} `json:"beans"`
}

func getBeans(url string, jmx *jmx) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	j := json.NewDecoder(res.Body)
	if err := j.Decode(&jmx); err != nil {
		return err
	}
	return nil
}

func c_hbase_region() opentsdb.MultiDataPoint {
	if !hbrEnable.Enabled() {
		return nil
	}
	var jmx jmx
	if err := getBeans(hbrURL, &jmx); err != nil {
		slog.Errorln(err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	if len(jmx.Beans) > 0 && len(jmx.Beans[0]) > 0 {
		for k, v := range jmx.Beans[0] {
			if _, ok := v.(float64); ok {
				Add(&md, "hbase.region."+k, v, nil, metadata.Unknown, metadata.None, "")
			}
		}
	}
	return md
}

func hbrepInit() {
	update := hTestUrl(hbrepURL, &hbrepEnable)
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
	var jmx jmx
	if err := getBeans(hbrepURL, &jmx); err != nil {
		slog.Errorln(err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, section := range jmx.Beans {
		var tags opentsdb.TagSet
		for k, v := range section {
			if s, ok := v.(string); ok && k == "name" {
				if strings.HasPrefix(s, "hadoop:service=Replication,name=ReplicationSource for") {
					sa := strings.Split(s, " ")
					if len(sa) == 3 {
						tags = opentsdb.TagSet{"instance": sa[2]}
						break
					}
				}
			}
		}
		for k, v := range section {
			if _, ok := v.(float64); ok {
				Add(&md, "hbase.region."+k, v, tags, metadata.Unknown, metadata.None, "")
			}
		}
	}
	return md
}
