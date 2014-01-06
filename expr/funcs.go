package expr

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tsaf/expr/parse"
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
}

func queryDuration(query, duration string, F func(map[string]opentsdb.Point) float64) (r []*Result, err error) {
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
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
	s, err := req.Query("ny-devtsdb04:4242")
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

func Avg(query, duration string) ([]*Result, error) {
	return queryDuration(query, duration, avg)
}

// avg returns the mean of x.
func avg(dps map[string]opentsdb.Point) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

func Dev(query, duration string) ([]*Result, error) {
	return queryDuration(query, duration, dev)
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

func Recent(query, duration string) ([]*Result, error) {
	return queryDuration(query, duration, recent)
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

func Since(query, duration string) ([]*Result, error) {
	return queryDuration(query, duration, since)
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
