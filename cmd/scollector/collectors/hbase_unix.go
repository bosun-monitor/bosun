// +build darwin linux

package collectors

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_hbase_region, Enable: enableURL(hbURL)})
	collectors = append(collectors, &IntervalCollector{F: c_hbase_replication, Enable: enableURL(hbRepURL)})
}

const (
	hbURL    = "http://localhost:60030/jmx?qry=hadoop:service=RegionServer,name=RegionServerStatistics"
	hbRepURL = "http://localhost:60030/jmx?qry=hadoop:service=Replication,name=*"
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

func c_hbase_region() opentsdb.MultiDataPoint {
	var jmx jmx
	if err := getBeans(hbURL, &jmx); err != nil {
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

func c_hbase_replication() opentsdb.MultiDataPoint {
	var jmx jmx
	if err := getBeans(hbRepURL, &jmx); err != nil {
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
