package collectors

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_railgun, init: rgInit})
}

var rgEnabled bool

const rgURL = "http://localhost:24088/"

func rgInit() {
	update := func() {
		resp, err := http.Get(rgURL)
		if err != nil {
			hbaseEnabled = false
			return
		}
		resp.Body.Close()
		rgEnabled = resp.StatusCode == 200
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_railgun() opentsdb.MultiDataPoint {
	if !rgEnabled {
		return nil
	}
	var md opentsdb.MultiDataPoint
	res, err := http.Get(rgURL)
	if err != nil {
		slog.Errorln(err)
		return nil
	}
	defer res.Body.Close()
	var r map[string]interface{}
	j := json.NewDecoder(res.Body)
	if err := j.Decode(&r); err != nil {
		slog.Errorln(err)
		return nil
	}
	slog.Infoln(r)
	for k, v := range r {
		if _, ok := v.(float64); ok {
			Add(&md, "railgun."+k, v, nil)
		}
	}

	return md
}
