package expr

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/tsaf/expr/parse"
	"github.com/StackExchange/tsaf/search"
	"github.com/StackExchange/tsaf/third_party/github.com/GaryBoone/GoStats/stats"
	"github.com/StackExchange/tsaf/third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/third_party/github.com/StackExchange/scollector/opentsdb"
)

var Builtins = map[string]parse.Func{
	"abs": {
		[]parse.FuncType{parse.TYPE_NUMBER},
		parse.TYPE_NUMBER,
		Abs,
	},
	"avg": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Avg,
	},
	"sum": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Sum,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_SCALAR},
		parse.TYPE_SERIES,
		Band,
	},
	"count": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_STRING},
		parse.TYPE_SCALAR,
		Count,
	},
	"change": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		Change,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Dev,
	},
	"drop": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING, parse.TYPE_SCALAR},
		parse.TYPE_SERIES,
		Drop,
	},
	"len": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Length,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Recent,
	},
	"since": {
		[]parse.FuncType{parse.TYPE_SERIES},
		parse.TYPE_NUMBER,
		Since,
	},
	"forecastlr": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_SCALAR},
		parse.TYPE_NUMBER,
		Forecast_lr,
	},
	"percentile": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_SCALAR},
		parse.TYPE_NUMBER,
		Percentile,
	},
	"q": {
		[]parse.FuncType{parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_STRING},
		parse.TYPE_SERIES,
		Query,
	},
	"t": {
		[]parse.FuncType{parse.TYPE_NUMBER, parse.TYPE_STRING},
		parse.TYPE_SERIES,
		Transpose,
	},
	"ungroup": {
		[]parse.FuncType{parse.TYPE_NUMBER},
		parse.TYPE_SCALAR,
		Ungroup,
	},
}

const tsdbFmt = "2006/01/02 15:04:05"

func Band(e *state, T miniprofiler.Timer, query, duration, period string, num float64) (r []*Result, err error) {
	T.Step("band", func(T miniprofiler.Timer) {
		var d, p opentsdb.Duration
		d, err = opentsdb.ParseDuration(duration)
		if err != nil {
			return
		}
		p, err = opentsdb.ParseDuration(period)
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
		if err = ExpandSearch(q); err != nil {
			return
		}
		req := opentsdb.Request{
			Queries: []*opentsdb.Query{q},
		}
		now := time.Now().UTC()
		for i := 0; i < int(num); i++ {
			now = now.Add(time.Duration(-p))
			req.End = now.Unix()
			req.Start = now.Add(time.Duration(-d)).Unix()
			e.addRequest(req)
			b, _ := json.MarshalIndent(&req, "", "  ")
			var s opentsdb.ResponseSet
			T.StepCustomTiming("tsdb", "query", string(b), func() {
				s, err = e.context.Query(req)
			})
			if err != nil {
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
	})
	return
}

func Query(e *state, T miniprofiler.Timer, query, sduration, eduration string) (r []*Result, err error) {
	q, err := opentsdb.ParseQuery(query)
	if err != nil {
		return
	}
	if err = ExpandSearch(q); err != nil {
		return
	}
	sd, err := opentsdb.ParseDuration(sduration)
	if err != nil {
		return
	}
	req := opentsdb.Request{
		Queries: []*opentsdb.Query{q},
		Start:   fmt.Sprintf("%s-ago", sd),
	}
	if eduration != "" {
		var ed opentsdb.Duration
		ed, err = opentsdb.ParseDuration(eduration)
		if err != nil {
			return
		}
		req.End = fmt.Sprintf("%s-ago", ed)
	}
	e.addRequest(req)
	b, _ := json.MarshalIndent(&req, "", "  ")
	var s opentsdb.ResponseSet
	T.StepCustomTiming("tsdb", "query", string(b), func() {
		s, err = e.context.Query(req)
	})
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

func Change(e *state, T miniprofiler.Timer, query, sduration, eduration string) (r []*Result, err error) {
	sd, err := opentsdb.ParseDuration(sduration)
	if err != nil {
		return
	}
	var ed opentsdb.Duration
	if eduration != "" {
		ed, err = opentsdb.ParseDuration(eduration)
		if err != nil {
			return
		}
	}
	r, err = Query(e, T, query, sduration, eduration)
	if err != nil {
		return
	}
	r, err = reduce(e, T, r, change, (sd - ed).Seconds())
	return
}

func Drop(e *state, T miniprofiler.Timer, series []*Result, op string, num float64) ([]*Result, error) {
	var res []*Result
	for _, s := range series {
		if t, ok := s.Value.(Series); ok {
			if len(t) == 0 {
				continue
			}
			m := make(Series)
			for ts, v := range t {
				if operate(op, num, float64(v)) == 1 {
					m[ts] = v
				}
			}
			res = append(res, &Result{
				Value:        m,
				Computations: s.Computations,
				Group:        s.Group,
			})
		} else {
			panic(fmt.Errorf("expr: expected a series"))
		}
	}
	return res, nil
}

func change(dps Series, args ...float64) float64 {
	return avg(dps) * args[0]
}

func reduce(e *state, T miniprofiler.Timer, series []*Result, F func(Series, ...float64) float64, args ...float64) ([]*Result, error) {
	var res []*Result
	for _, s := range series {
		switch t := s.Value.(type) {
		case Series:
			if len(t) == 0 {
				continue
			}
			s.Value = Number(F(t, args...))
			res = append(res, s)
		default:
			panic(fmt.Errorf("expr: expected a series"))
		}
	}
	return res, nil
}

func ExpandSearch(q *opentsdb.Query) error {
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

func Abs(e *state, T miniprofiler.Timer, series []*Result) []*Result {
	for _, s := range series {
		s.Value = Number(math.Abs(float64(s.Value.Value().(Number))))
	}
	return series
}

func Avg(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, avg)
}

// avg returns the mean of x.
func avg(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

func Count(e *state, T miniprofiler.Timer, query, sduration, eduration string) (r []*Result, err error) {
	r, err = Query(e, T, query, sduration, eduration)
	if err != nil {
		return
	}
	return []*Result{&Result{
		Value: Scalar(len(r)),
	}}, nil
}

func Sum(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, sum)
}

func sum(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	return
}

func Dev(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, dev)
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

func Length(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, length)
}

func length(dps Series, args ...float64) (a float64) {
	return float64(len(dps))
}

func Recent(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, recent)
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

func Since(e *state, T miniprofiler.Timer, series []*Result) ([]*Result, error) {
	return reduce(e, T, series, since)
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

func Forecast_lr(e *state, T miniprofiler.Timer, series []*Result, y float64) (r []*Result, err error) {
	return reduce(e, T, series, forecast_lr, y)
}

// forecast_lr returns the number of seconds a linear regression predicts the
// series will take to reach y_val.
func forecast_lr(dps Series, args ...float64) float64 {
	const tenYears = time.Hour * 24 * 365 * 10
	yVal := args[0]
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
	it := (yVal - intercept) / slope
	var i64 int64
	if it < math.MinInt64 {
		i64 = math.MinInt64
	} else if it > math.MaxInt64 {
		i64 = math.MaxInt64
	} else if math.IsNaN(it) {
		i64 = time.Now().Unix()
	} else {
		i64 = int64(it)
	}
	t := time.Unix(i64, 0)
	s := -time.Since(t)
	if s < -tenYears {
		s = -tenYears
	} else if s > tenYears {
		s = tenYears
	}
	return s.Seconds()
}

func Percentile(e *state, T miniprofiler.Timer, series []*Result, p float64) (r []*Result, err error) {
	return reduce(e, T, series, percentile, p)
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

func Ungroup(e *state, T miniprofiler.Timer, d []*Result) ([]*Result, error) {
	if len(d) != 1 {
		return nil, fmt.Errorf("ungroup: requires exactly one group")
	}
	d[0].Group = nil
	return d, nil
}

func Transpose(e *state, T miniprofiler.Timer, d []*Result, gp string) ([]*Result, error) {
	gps := strings.Split(gp, ",")
	m := make(map[string]*Result)
	for _, v := range d {
		ts := make(opentsdb.TagSet)
		for k, v := range v.Group {
			for _, b := range gps {
				if k == b {
					ts[k] = v
				}
			}
		}
		if _, ok := m[ts.String()]; !ok {
			m[ts.String()] = &Result{
				Group: ts,
				Value: make(Series),
			}
		}
		switch t := v.Value.(type) {
		case Number:
			r := m[ts.String()]
			i := strconv.Itoa((len(r.Value.(Series))))
			r.Value.(Series)[i] = opentsdb.Point(t)
			r.Computations = append(r.Computations, v.Computations...)
		default:
			panic(fmt.Errorf("expr: expected a number"))
		}
	}
	r := make([]*Result, len(m))
	i := 0
	for _, res := range m {
		r[i] = res
		i++
	}
	return r, nil
}
