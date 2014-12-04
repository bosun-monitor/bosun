// +build darwin linux

package collectors

import (
	"encoding/json"
	"net/http"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_hbase_region, Enable: enableURL(hbURL)})
	collectors = append(collectors, &IntervalCollector{F: c_hbase_replication, Enable: enableURL(hbRepURL)})
	collectors = append(collectors, &IntervalCollector{F: c_hbase_gc, Enable: enableURL(hbGCURL)})
}

const (
	hbURL    = "http://localhost:60030/jmx?qry=hadoop:service=RegionServer,name=RegionServerStatistics"
	hbRepURL = "http://localhost:60030/jmx?qry=hadoop:service=Replication,name=*"
	hbGCURL  = "http://localhost:60030/jmx?qry=java.lang:type=GarbageCollector,name=*"
)

type jmx struct {
	Beans []map[string]interface{} `json:"beans"`
}

func getBeans(url string, jmx *jmx) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&jmx); err != nil {
		return err
	}
	return nil
}

func c_hbase_region() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbURL, &j); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	if len(j.Beans) > 0 && len(j.Beans[0]) > 0 {
		for k, v := range j.Beans[0] {
			if _, ok := v.(float64); ok {
				Add(&md, "hbase.region."+k, v, nil, metadata.Unknown, metadata.None, "")
			}
		}
	}
	return md, nil
}

func c_hbase_gc() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbGCURL, &j); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	const metric = "hbase.region.gc."
	for _, bean := range j.Beans {
		if name, ok := bean["Name"].(string); ok && name != "" {
			ts := opentsdb.TagSet{"name": name}
			for k, v := range bean {
				if _, ok := v.(float64); ok {
					switch k {
					case "CollectionCount":
						Add(&md, metric+k, v, ts, metadata.Counter, metadata.Count, "A counter for the number of times that garbage collection has been called.")
					case "CollectionTime":
						Add(&md, metric+k, v, ts, metadata.Counter, metadata.None, "The total amount of time spent in garbage collection.")
					}
				}
			}
		}
	}
	return md, nil
}

func c_hbase_replication() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbRepURL, &j); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, section := range j.Beans {
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
	return md, nil
}
