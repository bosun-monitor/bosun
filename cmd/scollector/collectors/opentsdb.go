package collectors

import (
	"encoding/json"
	"net/http"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_opentsdb, Enable: EnableURL(tsdbURL)})
}

const tsdbURL = "http://localhost:4242/api/stats"

func c_opentsdb() (opentsdb.MultiDataPoint, error) {
	resp, err := http.Get(tsdbURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var md, tmp opentsdb.MultiDataPoint
	if err := json.NewDecoder(resp.Body).Decode(&tmp); err != nil {
		return nil, err
	}
	for _, v := range tmp {
		delete(v.Tags, "host")
		AddTS(&md, v.Metric, v.Timestamp, v.Value, v.Tags, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}
