package expr

import (
	"fmt"
	"math"

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
		avg,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_QUERY, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_SERIES,
		[]interface{}{DefDuration, DefPeriod, DefNum},
		band,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		dev,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		recent,
	},
}

func queryDuration(query, duration string, F func([]float64) float64) (r []Result, err error) {
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
	} else if len(s) == 0 {
		err = fmt.Errorf("expr: no results returned: %s", query)
		return
	}
	for _, res := range s {
		if len(res.DPS) == 0 {
			// do something here?
			continue
		}
		var f []float64
		for _, p := range res.DPS {
			f = append(f, float64(p))
		}
		r = append(r, Result{
			Value: Value(F(f)),
			Group: res.Tags,
		})
	}
	return
}

func Avg(query, duration string) ([]Result, error) {
	return queryDuration(query, duration, avg)
}

// avg returns the mean of x.
func avg(x []float64) (a float64) {
	for _, v := range x {
		a += v
	}
	a /= float64(len(x))
	return
}

func Dev(query, duration string) ([]Result, error) {
	return queryDuration(query, duration, dev)
}

// dev returns the sample standard deviation of x.
func dev(x []float64) (d float64) {
	a := avg(x)
	for _, v := range x {
		d += math.Pow(v-a, 2)
	}
	// how should we handle len(x) == 1?
	d /= float64(len(x) - 1)
	return math.Sqrt(d)
}

func recent() {}
func band()   {}
