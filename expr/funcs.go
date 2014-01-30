package expr

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GaryBoone/GoStats/stats"
	"github.com/StackExchange/scollector/opentsdb"
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
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		nil,
		Avg,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_SERIES,
		[]interface{}{DefDuration, DefPeriod, DefNum},
		Band,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		nil,
		Dev,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		nil,
		Recent,
	},
	"since": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		nil,
		Since,
	},
	"forecastlr": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_NUMBER},
		parse.TYPE_NUMBER,
		nil,
		Forecast_lr,
	},
	"percentile": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_NUMBER},
		parse.TYPE_NUMBER,
		nil,
		Percentile,
	},
	"q": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING},
		parse.TYPE_SERIES,
		[]interface{}{DefDuration},
		Query,
	},
}

const tsdbFmt = "2006/01/02 15:04:05"

func Band(e *state, query, duration, period string, num float64) (r []*Result, err error) {
	var d, p time.Duration
	d, err = ParseDuration(duration)
	if err != nil {
		return
	}
	p, err = ParseDuration(period)
	if err != nil {
		return
	}
	if num < 1 || num > 100 {
		err = fmt.Errorf("expr: Band: num out of bounds")
	}
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
		return
	}
	if err = expandSearch(q); err != nil {
		return
	}
	req := opentsdb.Request{
		Queries: []*opentsdb.Query{q},
	}
	now := time.Now().UTC()
	for i := 0; i < int(num); i++ {
		now = now.Add(-p)
		req.End = now.Unix()
		req.Start = now.Add(-d).Unix()
		s, e := req.Query(e.host)
		if e != nil {
			err = e
			return
		}
		for _, res := range s {
			newarr := true
			for _, a := range r {
				if !a.Group.Equal(res.Tags) {
					continue
				}
				newarr = false
				values := a.Value.(Series)
				for k, v := range res.DPS {
					values[k] = v
				}
			}
			if newarr {
				values := make(Series)
				a := &Result{Group: res.Tags}
				for k, v := range res.DPS {
					values[k] = v
				}
				a.Value = values
				r = append(r, a)
			}
		}
	}
	return
}

func Query(e *state, query, duration string) (r []*Result, err error) {
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
		return
	}
	if err = expandSearch(q); err != nil {
		return
	}
	d, err := ParseDuration(duration)
	if err != nil {
		return
	}
	req := opentsdb.Request{
		Queries: []*opentsdb.Query{q},
		Start:   fmt.Sprintf("%dms-ago", d.Nanoseconds()/1e6),
	}
	s, err := req.Query(e.host)
	if err != nil {
		return
	}
	for _, res := range s {
		r = append(r, &Result{
			Value: Series(res.DPS),
			Group: res.Tags,
		})
	}
	return
}

func reduce(e *state, series []*Result, F func(Series, ...float64) float64, args ...float64) (r []*Result, err error) {
	for _, s := range series {
		switch t := s.Value.(type) {
		case Series:
			if len(t) == 0 {
				// do something here?
				continue
			}
			r = append(r, &Result{
				Value: Number(F(t, args...)),
				Group: s.Group,
			})
		default:
			panic(fmt.Errorf("expr: expected a series"))
		}
	}
	return
}

func expandSearch(q *opentsdb.Query) error {
	for k, ov := range q.Tags {
		v := ov
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
		if len(nvs) == 0 {
			return fmt.Errorf("expr: no tags matching %s=%s", k, ov)
		}
		q.Tags[k] = strings.Join(nvs, "|")
	}
	return nil
}

func Avg(e *state, series []*Result) ([]*Result, error) {
	return reduce(e, series, avg)
}

// avg returns the mean of x.
func avg(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

func Dev(e *state, series []*Result) ([]*Result, error) {
	return reduce(e, series, dev)
}

// dev returns the sample standard deviation of x.
func dev(dps Series, args ...float64) (d float64) {
	a := avg(dps)
	for _, v := range dps {
		d += math.Pow(float64(v)-a, 2)
	}
	// how should we handle len(x) == 1?
	d /= float64(len(dps) - 1)
	return math.Sqrt(d)
}

func Recent(e *state, series []*Result) ([]*Result, error) {
	return reduce(e, series, recent)
}

func recent(dps Series, args ...float64) (a float64) {
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

func Since(e *state, series []*Result) ([]*Result, error) {
	return reduce(e, series, since)
}

func since(dps Series, args ...float64) (a float64) {
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

func Forecast_lr(e *state, series []*Result, y float64) (r []*Result, err error) {
	return reduce(e, series, forecast_lr, y)
}

// forecast_lr returns the number of seconds a linear regression predicts the
// series will take to reach y_val.
func forecast_lr(dps Series, args ...float64) float64 {
	y_val := args[0]
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
	intercept_time := (y_val - intercept) / slope
	t := time.Unix(int64(intercept_time), 0)
	s := time.Since(t)
	return -s.Seconds()
}

func Percentile(e *state, series []*Result, p float64) (r []*Result, err error) {
	return reduce(e, series, percentile, p)
}

// percentile returns the value at the corresponding percentile between 0 and 1.
// Min and Max can be simulated using p <= 0 and p >= 1, respectively.
func percentile(dps Series, args ...float64) (a float64) {
	p := args[0]
	var x []float64
	for _, v := range dps {
		x = append(x, float64(v))
	}
	sort.Float64s(x)
	if p <= 0 {
		return x[0]
	}
	if p >= 1 {
		return x[len(x)-1]
	}
	i := p * float64(len(x)-1)
	i = math.Ceil(i)
	return x[int(i)]
}
