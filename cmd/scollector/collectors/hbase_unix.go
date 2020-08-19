// +build darwin linux
// note: this collector only works on hbase 1.0+

package collectors

import (
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"strings"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var (
	hbURL    = "/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=Server"
	hbRegURL = "/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=Regions"
	hbRepURL = "/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=Replication"
	hbICPURL = "/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=IPC"
	hbGCURL  = "/jmx?qry=java.lang:type=GarbageCollector,name=*"
)

func init() {
	registerInit(func(c *conf.Conf) {
		host := ""
		if c.HadoopHost != "" {
			host = "http://" + c.HadoopHost
		} else {
			host = "http://localhost:60030"
		}
		hbURL = host + hbURL
		hbRegURL = host + hbRegURL
		hbRepURL = host + hbRepURL
		hbGCURL = host + hbGCURL
		hbICPURL = host + hbICPURL
		collectors = append(collectors, &IntervalCollector{F: c_hbase_region, Enable: enableURL(hbURL)})
		if c.HbaseRegions {
			collectors = append(collectors, &IntervalCollector{F: c_hbase_regions, Enable: enableURL(hbRegURL)})
		}
		collectors = append(collectors, &IntervalCollector{F: c_hbase_replication, Enable: enableURL(hbRepURL)})
		collectors = append(collectors, &IntervalCollector{F: c_hbase_gc, Enable: enableURL(hbGCURL)})
		collectors = append(collectors, &IntervalCollector{F: c_hbase_ipc, Enable: enableURL(hbICPURL)})
	})
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
			if vv, ok := v.(float64); ok {
				if vv < math.MaxInt64 {
					Add(&md, "hbase.region."+k, v, nil, metadata.Unknown, metadata.None, "")
				}
			}
		}
	}
	return md, nil
}

func c_hbase_ipc() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbICPURL, &j); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	if len(j.Beans) > 0 {
		for k, v := range j.Beans[0] {
			if vv, ok := v.(float64); ok {
				if vv < math.MaxInt64 {
					Add(&md, "hbase.ipc."+k, v, nil, metadata.Unknown, metadata.None, "")
				}
			}
		}
	}
	return md, nil
}

func c_hbase_regions() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbRegURL, &j); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	if len(j.Beans) > 0 && len(j.Beans[0]) > 0 {
		for k, v := range j.Beans[0] {
			if vv, ok := v.(float64); ok {
				if vv > math.MaxInt64 {
					continue
				}
				kParts := strings.Split(k, "_")
				t := make(opentsdb.TagSet)
				var m string
				for i := 0; i+1 < len(kParts); i += 2 {
					if kParts[i] == "metric" {
						m = strings.Join(kParts[i+1:], "_")
						break
					}
					t[kParts[i]] = kParts[i+1]
				}
				if m == "" {
					continue
				}
				Add(&md, "hbase.regions."+m, v, t, metadata.Unknown, metadata.None, "")
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
				if vv, ok := v.(float64); ok {
					if vv < math.MaxInt64 {
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
	}
	return md, nil
}

func c_hbase_replication() (opentsdb.MultiDataPoint, error) {
	var j jmx
	if err := getBeans(hbRepURL, &j); err != nil {
		return nil, err
	}
	excludeReg, err := regexp.Compile("source.\\d")
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, section := range j.Beans {
		for k, v := range section {
			// source.[0-9] entries are for other hosts in the cluster
			if excludeReg.MatchString(k) {
				continue
			}
			// Strip "source." and "sink." from the metric names.
			shortName := strings.TrimPrefix(k, "source.")
			shortName = strings.TrimPrefix(shortName, "sink.")
			metric := "hbase.region." + shortName
			if vv, ok := v.(float64); ok {
				if vv < math.MaxInt64 {
					Add(&md, metric, v, nil, metadata.Unknown, metadata.None, "")
				}
			}
		}
	}
	return md, nil
}
