package collectors

import (
	"encoding/json"
	"net/http"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_opentsdb, Enable: enableURL(tsdbURL)})
}

const tsdbURL = "http://localhost:4242/api/stats"

func c_opentsdb() opentsdb.MultiDataPoint {
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
