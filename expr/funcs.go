package expr

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GaryBoone/GoStats/stats"
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tsaf/expr/parse"
	"github.com/StackExchange/tsaf/search"
)

const (
	DefDuration = "1h"
	DefPeriod   = "1w"
	DefNum      = 8
)

var Builtins = map[string]parse.Func{
	"avg": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		Avg,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_QUERY, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_SERIES,
		[]interface{}{DefDuration, DefPeriod, DefNum},
		nil,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		Dev,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		Recent,
	},
	"since": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration, "5m"},
		Since,
	},
	"forecastlr": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_NUMBER,
		nil,
		Forecast_lr,
	},
}

func queryDuration(host, query, duration string, F func(map[string]opentsdb.Point) float64) (r []*Result, err error) {
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
		return
	}
	expandSearch(q)
	d, err := ParseDuration(duration)
	if err != nil {
		return
	}
	req := opentsdb.Request{
		Queries: []*opentsdb.Query{q},
		Start:   fmt.Sprintf("%dms-ago", d.Nanoseconds()/1e6),
	}
	s, err := req.Query(host)
	if err != nil {
		return
	}
	for _, res := range s {
		if len(res.DPS) == 0 {
			// do something here?
			continue
		}
		r = append(r, &Result{
			Value: Value(F(res.DPS)),
			Group: res.Tags,
		})
	}
	return
}

func expandSearch(q *opentsdb.Query) {
	for k, v := range q.Tags {
		if v == "*" || !strings.Contains(v, "*") || strings.Contains(v, "|") {
			continue
		}
		v = strings.Replace(v, ".", `\.`, -1)
		v = strings.Replace(v, "*", ".*", -1)
		v = "^" + v + "$"
		re := regexp.MustCompile(v)
		var nvs []string
		vs := search.TagValuesByMetricTagKey(q.Metric, k)
		for _, nv := range vs {
			if re.MatchString(nv) {
				nvs = append(nvs, nv)
			}
		}
		q.Tags[k] = strings.Join(nvs, "|")
	}
}

func Avg(host, query, duration string) ([]*Result, error) {
	return queryDuration(host, query, duration, avg)
}

// avg returns the mean of x.
func avg(dps map[string]opentsdb.Point) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

func Dev(host, query, duration string) ([]*Result, error) {
	return queryDuration(host, query, duration, dev)
}

// dev returns the sample standard deviation of x.
func dev(dps map[string]opentsdb.Point) (d float64) {
	a := avg(dps)
	for _, v := range dps {
		d += math.Pow(float64(v)-a, 2)
	}
	// how should we handle len(x) == 1?
	d /= float64(len(dps) - 1)
	return math.Sqrt(d)
}

func Recent(host, query, duration string) ([]*Result, error) {
	return queryDuration(host, query, duration, recent)
}

func recent(dps map[string]opentsdb.Point) (a float64) {
	last := -1
	for k, v := range dps {
		d, err := strconv.Atoi(k)
		if err != nil {
			panic(err)
		}
		if d > last {
			a = float64(v)
		}
	}
	return
}

func Since(host, query, duration string) ([]*Result, error) {
	return queryDuration(host, query, duration, since)
}

func since(dps map[string]opentsdb.Point) (a float64) {
	var last time.Time
	for k := range dps {
		d, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			panic(err)
		}
		t := time.Unix(d, 0)
		if t.After(last) {
			last = t
		}
	}
	s := time.Since(last)
	return s.Seconds()
}

//forecast_lr Returns the number of seconds until the the series will have value Y according to a
//Linear Regression
func Forecast_lr(host, query, duration string, y float64) (r []*Result, err error) {
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
		return
	}
	expandSearch(q)
	d, err := ParseDuration(duration)
	if err != nil {
		return
	}
	req := opentsdb.Request{
		Queries: []*opentsdb.Query{q},
		Start:   fmt.Sprintf("%dms-ago", d.Nanoseconds()/1e6),
	}
	s, err := req.Query(host)
	if err != nil {
		return
	}
	for _, res := range s {
		if len(res.DPS) == 0 {
			// do something here?
			continue
		}
		r = append(r, &Result{
			Value: Value(forecast_lr(res.DPS, y)),
			Group: res.Tags,
		})
	}
	return
}

func forecast_lr(dps map[string]opentsdb.Point, y_val float64) (a float64) {
	var x []float64
	var y []float64
	for k, v := range dps {
		d, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			panic(err)
		}
		x = append(x, float64(d))
		y = append(y, float64(v))
	}
	var slope, intercept, _, _, _, _ = stats.LinearRegression(x, y)
	// If the slope is basically 0, return -1 since forecast alerts wouldn't care about things that
	// "already happened". There might be a better way to handle this, but this works for now
	if int64(slope) == 0 {
		return -1
	}
	//Apparently it is okay for slope to be Zero, there is no divide by zero, not sure why
	intercept_time := (y_val - intercept) / slope
	t := time.Unix(int64(intercept_time), 0)
	s := time.Since(t)
	return -s.Seconds()

}
