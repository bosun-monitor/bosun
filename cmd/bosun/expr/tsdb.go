package expr

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
)

// TSDB defines functions for use with an OpenTSDB backend.
var TSDB = map[string]parse.Func{
	"band": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   tagQuery,
		F:      Band,
	},
	"shiftBand": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   tagQuery,
		F:      ShiftBand,
	},
	"over": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   tagQuery,
		F:      Over,
	},
	"change": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeNumberSet,
		Tags:   tagQuery,
		F:      Change,
	},
	"count": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeScalar,
		F:      Count,
	},
	"q": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagQuery,
		F:      Query,
	},
	"window": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagQuery,
		F:      Window,
		Check:  windowCheck,
	},
}

const tsdbMaxTries = 3

func timeTSDBRequest(e *State, T miniprofiler.Timer, req *opentsdb.Request) (s opentsdb.ResponseSet, err error) {
	e.tsdbQueries = append(e.tsdbQueries, *req)
	if e.autods > 0 {
		for _, q := range req.Queries {
			if q.Downsample == "" {
				if err := req.AutoDownsample(e.autods); err != nil {
					return nil, err
				}
			}
		}
	}
	b, _ := json.MarshalIndent(req, "", "  ")
	tries := 1
	for {
		T.StepCustomTiming("tsdb", "query", string(b), func() {
			getFn := func() (interface{}, error) {
				return e.TSDBContext.Query(req)
			}
			var val interface{}
			var hit bool
			val, err, hit = e.Cache.Get(string(b), getFn)
			collectCacheHit(e.Cache, "opentsdb", hit)
			rs := val.(opentsdb.ResponseSet)
			s = rs.Copy()
			for _, r := range rs {
				if r.SQL != "" {
					T.AddCustomTiming("sql", "query", time.Now(), time.Now(), r.SQL)
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

func bandTSDB(e *State, T miniprofiler.Timer, query, duration, period string, num float64, rfunc func(*Results, *opentsdb.Response, time.Duration) error) (r *Results, err error) {
	r = new(Results)
	r.IgnoreOtherUnjoined = true
	r.IgnoreUnjoined = true
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
			err = fmt.Errorf("num out of bounds")
		}
		var q *opentsdb.Query
		q, err = opentsdb.ParseQuery(query, e.TSDBContext.Version())
		if err != nil {
			return
		}
		if !e.TSDBContext.Version().FilterSupport() {
			if err = e.Search.Expand(q); err != nil {
				return
			}
		}
		req := opentsdb.Request{
			Queries: []*opentsdb.Query{q},
		}
		now := e.now
		req.End = now.Unix()
		req.Start = now.Add(time.Duration(-d)).Unix()
		if err = req.SetTime(e.now); err != nil {
			return
		}
		for i := 0; i < int(num); i++ {
			now = now.Add(time.Duration(-p))
			req.End = now.Unix()
			req.Start = now.Add(time.Duration(-d)).Unix()
			var s opentsdb.ResponseSet
			s, err = timeTSDBRequest(e, T, &req)
			if err != nil {
				return
			}
			for _, res := range s {
				if e.Squelched(res.Tags) {
					continue
				}
				//offset := e.now.Sub(now.Add(time.Duration(p-d)))
				offset := e.now.Sub(now)
				if err = rfunc(r, res, offset); err != nil {
					return
				}
			}
		}
	})
	return
}

func Window(e *State, T miniprofiler.Timer, query, duration, period string, num float64, rfunc string) (*Results, error) {
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
	bandFn := func(results *Results, resp *opentsdb.Response, offset time.Duration) error {
		values := make(Series)
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
		callResult := &Results{
			Results: ResultSlice{
				&Result{
					Group: resp.Tags,
					Value: values,
				},
			},
		}
		fnArgs := []reflect.Value{reflect.ValueOf(e), reflect.ValueOf(T), reflect.ValueOf(callResult)}
		if isPerc {
			fnArgs = append(fnArgs, reflect.ValueOf(fromScalar(percValue)))
		}
		fnResult := windowFn.Call(fnArgs)
		if !fnResult[1].IsNil() {
			if err := fnResult[1].Interface().(error); err != nil {
				return err
			}
		}
		minTime := time.Unix(min, 0).UTC()
		fres := float64(fnResult[0].Interface().(*Results).Results[0].Value.(Number))
		found := false
		for _, result := range results.Results {
			if result.Group.Equal(resp.Tags) {
				found = true
				v := result.Value.(Series)
				v[minTime] = fres
				break
			}
		}
		if !found {
			results.Results = append(results.Results, &Result{
				Group: resp.Tags,
				Value: Series{
					minTime: fres,
				},
			})
		}
		return nil
	}
	r, err := bandTSDB(e, T, query, duration, period, num, bandFn)
	if err != nil {
		err = fmt.Errorf("expr: Window: %v", err)
	}
	return r, err
}

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

func Band(e *State, T miniprofiler.Timer, query, duration, period string, num float64) (r *Results, err error) {
	r, err = bandTSDB(e, T, query, duration, period, num, func(r *Results, res *opentsdb.Response, offset time.Duration) error {
		newarr := true
		for _, a := range r.Results {
			if !a.Group.Equal(res.Tags) {
				continue
			}
			newarr = false
			values := a.Value.(Series)
			for k, v := range res.DPS {
				i, e := strconv.ParseInt(k, 10, 64)
				if e != nil {
					return e
				}
				values[time.Unix(i, 0).UTC()] = float64(v)
			}
		}
		if newarr {
			values := make(Series)
			a := &Result{Group: res.Tags}
			for k, v := range res.DPS {
				i, e := strconv.ParseInt(k, 10, 64)
				if e != nil {
					return e
				}
				values[time.Unix(i, 0).UTC()] = float64(v)
			}
			a.Value = values
			r.Results = append(r.Results, a)
		}
		return nil
	})
	if err != nil {
		err = fmt.Errorf("expr: Band: %v", err)
	}
	return
}

func ShiftBand(e *State, T miniprofiler.Timer, query, duration, period string, num float64) (r *Results, err error) {
	r, err = bandTSDB(e, T, query, duration, period, num, func(r *Results, res *opentsdb.Response, offset time.Duration) error {
		values := make(Series)
		a := &Result{Group: res.Tags.Merge(opentsdb.TagSet{"shift": offset.String()})}
		for k, v := range res.DPS {
			i, e := strconv.ParseInt(k, 10, 64)
			if e != nil {
				return e
			}
			values[time.Unix(i, 0).Add(offset).UTC()] = float64(v)
		}
		a.Value = values
		r.Results = append(r.Results, a)
		return nil
	})
	if err != nil {
		err = fmt.Errorf("expr: Band: %v", err)
	}
	return
}

func Over(e *State, T miniprofiler.Timer, query, duration, period string, num float64) (r *Results, err error) {
	r = new(Results)
	r.IgnoreOtherUnjoined = true
	r.IgnoreUnjoined = true
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
			err = fmt.Errorf("num out of bounds")
		}
		var q *opentsdb.Query
		q, err = opentsdb.ParseQuery(query, e.TSDBContext.Version())
		if err != nil {
			return
		}
		if !e.TSDBContext.Version().FilterSupport() {
			if err = e.Search.Expand(q); err != nil {
				return
			}
		}
		req := opentsdb.Request{
			Queries: []*opentsdb.Query{q},
		}
		now := e.now
		req.End = now.Unix()
		req.Start = now.Add(time.Duration(-d)).Unix()
		for i := 0; i < int(num); i++ {
			var s opentsdb.ResponseSet
			s, err = timeTSDBRequest(e, T, &req)
			if err != nil {
				return
			}
			offset := e.now.Sub(now)
			for _, res := range s {
				if e.Squelched(res.Tags) {
					continue
				}
				values := make(Series)
				a := &Result{Group: res.Tags.Merge(opentsdb.TagSet{"shift": offset.String()})}
				for k, v := range res.DPS {
					i, err := strconv.ParseInt(k, 10, 64)
					if err != nil {
						return
					}
					values[time.Unix(i, 0).Add(offset).UTC()] = float64(v)
				}
				a.Value = values
				r.Results = append(r.Results, a)
			}
			now = now.Add(time.Duration(-p))
			req.End = now.Unix()
			req.Start = now.Add(time.Duration(-d)).Unix()
		}
	})
	return
}

func Query(e *State, T miniprofiler.Timer, query, sduration, eduration string) (r *Results, err error) {
	r = new(Results)
	q, err := opentsdb.ParseQuery(query, e.TSDBContext.Version())
	if q == nil && err != nil {
		return
	}
	if !e.TSDBContext.Version().FilterSupport() {
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
	if err = req.SetTime(e.now); err != nil {
		return
	}
	s, err = timeTSDBRequest(e, T, &req)
	if err != nil {
		return
	}
	for _, res := range s {
		if e.Squelched(res.Tags) {
			continue
		}
		values := make(Series)
		for k, v := range res.DPS {
			i, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			values[time.Unix(i, 0).UTC()] = float64(v)
		}
		r.Results = append(r.Results, &Result{
			Value: values,
			Group: res.Tags,
		})
	}
	return
}

func Change(e *State, T miniprofiler.Timer, query, sduration, eduration string) (r *Results, err error) {
	r = new(Results)
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
	r, err = reduce(e, T, r, change, fromScalar((sd - ed).Seconds()))
	return
}

func change(dps Series, args ...float64) float64 {
	return avg(dps) * args[0]
}
