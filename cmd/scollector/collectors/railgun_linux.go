package collectors

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_railgun, init: rgInit})
}

var (
	rgEnable   bool
	rgLock     sync.Mutex
	rgListenRE = regexp.MustCompile(`^stats.listen\s+?=\s+?([0-9.:]+)`)
	rgURL      string
)

func rgEnabled() (b bool) {
	rgLock.Lock()
	b = rgEnable
	rgLock.Unlock()
	return
}

func parseRailUrl() string {
	var config string
	var url string
	readCommand(func(line string) {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.Contains(fields[0], "rg-listener") {
			return
		}
		for i, s := range fields {
			if s == "-config" && len(fields) > i {
				config = fields[i+1]
			}
		}
	}, "ps", "-e", "-o", "args")
	if config == "" {
		return config
	}
	readLine(config, func(s string) {
		if m := rgListenRE.FindStringSubmatch(s); len(m) > 0 {
			url = "http://" + m[1]
		}
	})
	return url
}

func rgInit() {
	update := func() {
		rgURL = parseRailUrl()
		resp, err := http.Get(rgURL)
		rgLock.Lock()
		defer rgLock.Unlock()
		if err != nil {
			rgEnable = false
			return
		}
		resp.Body.Close()
		rgEnable = resp.StatusCode == 200
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
}

func c_railgun() opentsdb.MultiDataPoint {
	if !rgEnabled() {
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
	for k, v := range r {
		if _, ok := v.(float64); ok {
			Add(&md, "railgun."+k, v, nil)
		}
	}
	return md
}
