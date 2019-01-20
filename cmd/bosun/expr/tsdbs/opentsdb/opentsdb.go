// Package opentsdb contains the OpenTSDB query functions for the Bosun expression language.
package opentsdb

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	opentsdb "bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
)

// ExprFuncs defines Bosun expression functions for use with an OpenTSDB backend.
var ExprFuncs = map[string]parse.Func{
	"band": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       Band,
	},
	"bandQuery": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       BandQuery,
	},
	"shiftBand": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       ShiftBand,
	},
	"over": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       Over,
	},
	"overQuery": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       OverQuery,
	},
	"change": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return:  models.TypeNumberSet,
		TagKeys: tagQuery,
		F:       Change,
	},
	"count": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeScalar,
		F:      Count,
	},
	"q": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       Query,
	},
	"window": {
		Args:    []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: tagQuery,
		F:       Window,
		Check:   windowCheck,
	},
}

// tsdbMaxTries sets the number of times to retry an query if an error is
// returned from querying OpenTSDB.
const tsdbMaxTries = 3

// oTag Functions are for extracting expected tag keys from the arguments to the function
func tagQuery(args []parse.Node) (parse.TagKeys, error) {
	n := args[0].(*parse.StringNode)
	// Since all 2.1 queries are valid 2.2 queries, at this time
	// we can just use 2.2 to parse to identify group by tags
	q, err := opentsdb.ParseQuery(n.Text, opentsdb.Version2_2)
	if q == nil && err != nil {
		return nil, err
	}
	t := make(parse.TagKeys)
	for k := range q.GroupByTags {
		t[k] = struct{}{}
	}
	return t, nil
}

// timeRequest executes a request up to tsdbMaxTries and returns the ResponseSet.
func timeRequest(e *expr.State, req *opentsdb.Request) (s opentsdb.ResponseSet, err error) {
	e.OpenTSDBQueries = append(e.OpenTSDBQueries, *req)
	if e.AutoDS() > 0 {
		for _, q := range req.Queries {
			if q.Downsample == "" {
				if err := req.AutoDownsample(e.AutoDS()); err != nil {
					return nil, err
				}
			}
		}
	}
	b, _ := json.MarshalIndent(req, "", "  ")
	tries := 1
	for {
		e.Timer.StepCustomTiming("tsdb", "query", string(b), func() {
			getFn := func() (interface{}, error) {
				return e.OpenTSDB.Query(req)
			}
			var val interface{}
			var hit bool
			val, err, hit = e.Cache.Get(string(b), getFn)
			expr.CollectCacheHit(e.Cache, "opentsdb", hit)
			rs := val.(opentsdb.ResponseSet)
			s = rs.Copy()
			for _, r := range rs {
				if r.SQL != "" {
					e.Timer.AddCustomTiming("sql", "query", time.Now(), time.Now(), r.SQL)
				}
			}
		})
		if err == nil || tries == tsdbMaxTries {
			break
		}
		slog.Errorf("Error on tsdb query %d: %s", tries, err.Error())
		tries++
	}
	return
}

func bandTSDB(e *expr.State, query, duration, period, eduration string, num float64, rfunc func(*expr.ValueSet, *opentsdb.Response, time.Duration) error) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
	r.IgnoreOtherUnjoined = true
	r.IgnoreUnjoined = true
	e.Timer.Step("band", func(T miniprofiler.Timer) {
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
			err = fmt.Errorf("num out of bounds")
		}
		var q *opentsdb.Query
		q, err = opentsdb.ParseQuery(query, e.OpenTSDB.Version())
		if err != nil {
			return
		}
		if !e.OpenTSDB.Version().FilterSupport() {
			if err = e.Search.Expand(q); err != nil {
				return
			}
		}
		req := opentsdb.Request{
			Queries: []*opentsdb.Query{q},
		}
		end := e.Now()
		if eduration != "" {
			var ed opentsdb.Duration
			ed, err = opentsdb.ParseDuration(eduration)
			if err != nil {
				return
			}
			end = end.Add(time.Duration(-ed))
		}
		req.End = end.Unix()
		req.Start = end.Add(time.Duration(-d)).Unix()
		if err = req.SetTime(e.Now()); err != nil {
			return
		}
		for i := 0; i < int(num); i++ {
			req.End = end.Unix()
			req.Start = end.Add(time.Duration(-d)).Unix()
			var s opentsdb.ResponseSet
			s, err = timeRequest(e, &req)
			if err != nil {
				return
			}
			for _, res := range s {
				if e.Squelched(res.Tags) {
					continue
				}
				//offset := e.Now().Sub(now.Add(time.Duration(p-d)))
				offset := e.Now().Sub(end)
				if err = rfunc(r, res, offset); err != nil {
					return
				}
			}
			end = end.Add(time.Duration(-p))
		}
	})
	return
}

// Window maps to the "window" function in the expression language.
func Window(e *expr.State, query, duration, period string, num float64, rfunc string) (*expr.ValueSet, error) {
	var isPerc bool
	var percValue float64
	if len(rfunc) > 0 && rfunc[0] == 'p' {
		var err error
		percValue, err = strconv.ParseFloat(rfunc[1:], 10)
		isPerc = err == nil
	}
	if isPerc {
		if percValue < 0 || percValue > 1 {
			return nil, fmt.Errorf("expr: window: percentile number must be greater than or equal to zero 0 and less than or equal 1")
		}
		rfunc = "percentile"
	}
	fn, ok := e.GetFunction(rfunc)
	if !ok {
		return nil, fmt.Errorf("expr: Window: no %v function", rfunc)
	}
	windowFn := reflect.ValueOf(fn.F)
	bandFn := func(results *expr.ValueSet, resp *opentsdb.Response, offset time.Duration) error {
		values := make(expr.Series)
		min := int64(math.MaxInt64)
		for k, v := range resp.DPS {
			i, e := strconv.ParseInt(k, 10, 64)
			if e != nil {
				return e
			}
			if i < min {
				min = i
			}
			values[time.Unix(i, 0).UTC()] = float64(v)
		}
		if len(values) == 0 {
			return nil
		}
		callResult := &expr.ValueSet{
			Elements: expr.ElementSlice{
				&expr.Element{
					Group: resp.Tags,
					Value: values,
				},
			},
		}
		fnArgs := []reflect.Value{reflect.ValueOf(e), reflect.ValueOf(callResult)}
		if isPerc {
			fnArgs = append(fnArgs, reflect.ValueOf(expr.FromScalar(percValue)))
		}
		fnResult := windowFn.Call(fnArgs)
		if !fnResult[1].IsNil() {
			if err := fnResult[1].Interface().(error); err != nil {
				return err
			}
		}
		minTime := time.Unix(min, 0).UTC()
		fres := float64(fnResult[0].Interface().(*expr.ValueSet).Elements[0].Value.(expr.Number))
		found := false
		for _, result := range results.Elements {
			if result.Group.Equal(resp.Tags) {
				found = true
				v := result.Value.(expr.Series)
				v[minTime] = fres
				break
			}
		}
		if !found {
			results.Elements = append(results.Elements, &expr.Element{
				Group: resp.Tags,
				Value: expr.Series{
					minTime: fres,
				},
			})
		}
		return nil
	}
	r, err := bandTSDB(e, query, duration, period, period, num, bandFn)
	if err != nil {
		err = fmt.Errorf("expr: Window: %v", err)
	}
	return r, err
}

// windowCheck checks the validty of the "window" function in the expression language
// for validity at parse time.
func windowCheck(t *parse.Tree, f *parse.FuncNode) error {
	name := f.Args[4].(*parse.StringNode).Text
	var isPerc bool
	var percValue float64
	if len(name) > 0 && name[0] == 'p' {
		var err error
		percValue, err = strconv.ParseFloat(name[1:], 10)
		isPerc = err == nil
	}
	if isPerc {
		if percValue < 0 || percValue > 1 {
			return fmt.Errorf("expr: window: percentile number must be greater than or equal to zero 0 and less than or equal 1")
		}
		return nil
	}
	v, ok := t.GetFunction(name)
	if !ok {
		return fmt.Errorf("expr: Window: unknown function %v", name)
	}
	if len(v.Args) != 1 || v.Args[0] != models.TypeSeriesSet || v.Return != models.TypeNumberSet {
		return fmt.Errorf("expr: Window: %v is not a reduction function", name)
	}
	return nil
}

// BandQuery maps to the "bandQuery" function in the expression language.
func BandQuery(e *expr.State, query, duration, period, eduration string, num float64) (r *expr.ValueSet, err error) {
	r, err = bandTSDB(e, query, duration, period, eduration, num, func(r *expr.ValueSet, res *opentsdb.Response, offset time.Duration) error {
		newarr := true
		for _, a := range r.Elements {
			if !a.Group.Equal(res.Tags) {
				continue
			}
			newarr = false
			values := a.Value.(expr.Series)
			for k, v := range res.DPS {
				i, e := strconv.ParseInt(k, 10, 64)
				if e != nil {
					return e
				}
				values[time.Unix(i, 0).UTC()] = float64(v)
			}
		}
		if newarr {
			values := make(expr.Series)
			a := &expr.Element{Group: res.Tags}
			for k, v := range res.DPS {
				i, e := strconv.ParseInt(k, 10, 64)
				if e != nil {
					return e
				}
				values[time.Unix(i, 0).UTC()] = float64(v)
			}
			a.Value = values
			r.Elements = append(r.Elements, a)
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("expr: Band: %v", err)
	}
	return
}

// OverQuery maps to the "overQuery" function in the expression language.
func OverQuery(e *expr.State, query, duration, period, eduration string, num float64) (r *expr.ValueSet, err error) {
	r, err = bandTSDB(e, query, duration, period, eduration, num, func(r *expr.ValueSet, res *opentsdb.Response, offset time.Duration) error {
		values := make(expr.Series)
		a := &expr.Element{Group: res.Tags.Merge(opentsdb.TagSet{"shift": offset.String()})}
		for k, v := range res.DPS {
			i, e := strconv.ParseInt(k, 10, 64)
			if e != nil {
				return e
			}
			values[time.Unix(i, 0).Add(offset).UTC()] = float64(v)
		}
		a.Value = values
		r.Elements = append(r.Elements, a)
		return nil
	})
	if err != nil {
		err = fmt.Errorf("expr: Band: %v", err)
	}
	return
}

// Band maps to the "band" function in the expression language.
func Band(e *expr.State, query, duration, period string, num float64) (r *expr.ValueSet, err error) {
	// existing Band behavior is to end 'period' ago, so pass period as eduration.
	return BandQuery(e, query, duration, period, period, num)
}

// ShiftBand maps to the "shiftBand" function in the expression language.
func ShiftBand(e *expr.State, query, duration, period string, num float64) (r *expr.ValueSet, err error) {
	return OverQuery(e, query, duration, period, period, num)
}

// Over maps to the "over" function in the expression language.
func Over(e *expr.State, query, duration, period string, num float64) (r *expr.ValueSet, err error) {
	return OverQuery(e, query, duration, period, "", num)
}

// Query maps to the "q" function in the expression language.
func Query(e *expr.State, query, sduration, eduration string) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
	q, err := opentsdb.ParseQuery(query, e.OpenTSDB.Version())
	if q == nil && err != nil {
		return
	}
	if !e.OpenTSDB.Version().FilterSupport() {
		if err = e.Search.Expand(q); err != nil {
			return
		}
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
	var s opentsdb.ResponseSet
	if err = req.SetTime(e.Now()); err != nil {
		return
	}
	s, err = timeRequest(e, &req)
	if err != nil {
		return
	}
	for _, res := range s {
		if e.Squelched(res.Tags) {
			continue
		}
		values := make(expr.Series)
		for k, v := range res.DPS {
			i, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			values[time.Unix(i, 0).UTC()] = float64(v)
		}
		r.Elements = append(r.Elements, &expr.Element{
			Value: values,
			Group: res.Tags,
		})
	}
	return
}

// Change maps to the "change" function in the expression language.
func Change(e *expr.State, query, sduration, eduration string) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
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
	r, err = Query(e, query, sduration, eduration)
	if err != nil {
		return
	}
	r, err = expr.ReduceSeriesSet(e, r, change, expr.FromScalar((sd - ed).Seconds()))
	return
}

// change is the per series reduction function.
func change(dps expr.Series, args ...float64) float64 {
	return expr.SeriesAvg(dps) * args[0]
}

// Count maps to the "count" function in the expression language.
func Count(e *expr.State, query, sduration, eduration string) (r *expr.ValueSet, err error) {
	r, err = Query(e, query, sduration, eduration)
	if err != nil {
		return
	}
	return &expr.ValueSet{
		Elements: []*expr.Element{
			{Value: expr.Scalar(len(r.Elements))},
		},
	}, nil
}
