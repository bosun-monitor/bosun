package expr

import (
	"fmt"
	"math"
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"github.com/MiniProfiler/go/miniprofiler"
)

var filterFuncs = parse.FuncMap{
	"crop": {
		Args:   cropDoc.Arguments.TypeSlice(),
		Return: cropDoc.Return,
		Tags:   tagFirst,
		F:      Crop,
		Doc:    cropDoc,
	},
	"dropge": {
		Args:   dropGeDoc.Arguments.TypeSlice(),
		Return: dropGeDoc.Return,
		Tags:   tagFirst,
		F:      DropGe,
		Doc:    dropGeDoc,
	},
	"dropg": {
		Args:   dropGDoc.Arguments.TypeSlice(),
		Return: dropGDoc.Return,
		Tags:   tagFirst,
		F:      DropG,
		Doc:    dropGDoc,
	},
	"drople": {
		Args:   dropLeDoc.Arguments.TypeSlice(),
		Return: dropLeDoc.Return,
		Tags:   tagFirst,
		F:      DropLe,
		Doc:    dropLeDoc,
	},
	"dropl": {
		Args:   dropLDoc.Arguments.TypeSlice(),
		Return: dropLDoc.Return,
		Tags:   tagFirst,
		F:      DropL,
		Doc:    dropLDoc,
	},
	"dropna": {
		Args:   dropNADoc.Arguments.TypeSlice(),
		Return: dropNADoc.Return,
		Tags:   tagFirst,
		F:      DropNA,
		Doc:    dropNADoc,
	},
	"dropbool": {
		Args:   dropBoolDoc.Arguments.TypeSlice(),
		Return: dropBoolDoc.Return,
		Tags:   tagFirst,
		F:      DropBool,
		Doc:    dropBoolDoc,
	},
	"filter": {
		Args:   filterDoc.Arguments.TypeSlice(),
		Return: filterDoc.Return,
		Tags:   tagFirst,
		F:      Filter,
		Doc:    filterDoc,
	},
	"limit": {
		Args:   limitDoc.Arguments.TypeSlice(),
		Return: limitDoc.Return,
		Tags:   tagFirst,
		F:      Limit,
		Doc:    limitDoc,
	},
	"tail": {
		Args:   tailDoc.Arguments.TypeSlice(),
		Return: tailDoc.Return,
		Tags:   tagFirst,
		F:      Tail,
		Doc:    tailDoc,
	},
}

var cropDoc = &doc.Func{
	Name:    "crop",
	Summary: "Returns a seriesSet where each series is has datapoints removed if the datapoint is before start (from now, in seconds) or after end (also from now, in seconds). This is useful if you want to alert on different timespans for different items in a set.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "start",
			Type: models.TypeNumberSet,
		},
		doc.Arg{
			Name: "end",
			Type: models.TypeNumberSet,
		},
	},
	Return:   models.TypeSeriesSet,
	Examples: []doc.HTMLString{doc.HTMLString(cropExampleOne)},
}

func Crop(e *State, T miniprofiler.Timer, sSet *Results, startSet *Results, endSet *Results) (*Results, error) {
	results := Results{}
INNER:
	for _, seriesResult := range sSet.Results {
		for _, startResult := range startSet.Results {
			for _, endResult := range endSet.Results {
				startHasNoGroup := len(startResult.Group) == 0
				endHasNoGroup := len(endResult.Group) == 0
				startOverlapsSeries := seriesResult.Group.Overlaps(startResult.Group)
				endOverlapsSeries := seriesResult.Group.Overlaps(endResult.Group)
				if (startHasNoGroup || startOverlapsSeries) && (endHasNoGroup || endOverlapsSeries) {
					res := crop(e, seriesResult, startResult, endResult)
					results.Results = append(results.Results, res)
					continue INNER
				}
			}
		}
	}
	return &results, nil
}

func crop(e *State, seriesResult *Result, startResult *Result, endResult *Result) *Result {
	startNumber := startResult.Value.(Number)
	endNumber := endResult.Value.(Number)
	start := e.now.Add(-time.Duration(time.Duration(startNumber) * time.Second))
	end := e.now.Add(-time.Duration(time.Duration(endNumber) * time.Second))
	series := seriesResult.Value.(Series)
	for timeStamp := range series {
		if timeStamp.Before(start) || timeStamp.After(end) {
			delete(series, timeStamp)
		}
	}
	return seriesResult
}

var dropArgs = doc.Arguments{
	sSeriesSetArg,
	doc.Arg{
		Name: "threshold",
		Type: models.TypeNumberSet,
	},
}

func dropValues(e *State, T miniprofiler.Timer, series *Results, threshold *Results, dropFunction func(float64, float64) bool) (*Results, error) {
	f := func(res *Results, s *Result, floats []float64) error {
		nv := make(Series)
		for k, v := range s.Value.Value().(Series) {
			if !dropFunction(float64(v), floats[0]) {
				//preserve values which should not be discarded
				nv[k] = v
			}
		}
		if len(nv) == 0 {
			return fmt.Errorf("series %s is empty", s.Group)
		}
		s.Value = nv
		res.Results = append(res.Results, s)
		return nil
	}
	return match(f, series, threshold)
}

var dropGeDoc = &doc.Func{
	Name:      "dropge",
	Summary:   "dropge removes any values greater than or equal to threshold from each series in s. Will error if this operation results in an empty series.",
	Arguments: dropArgs,
	Return:    models.TypeSeriesSet,
}

func DropGe(e *State, T miniprofiler.Timer, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value >= threshold }
	return dropValues(e, T, series, threshold, dropFunction)
}

var dropGDoc = &doc.Func{
	Name:      "dropg",
	Summary:   "dropg removes any values greater than threshold from each series in s. Will error if this operation results in an empty series.",
	Arguments: dropArgs,
	Return:    models.TypeSeriesSet,
}

func DropG(e *State, T miniprofiler.Timer, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value > threshold }
	return dropValues(e, T, series, threshold, dropFunction)
}

var dropLeDoc = &doc.Func{
	Name:      "drople",
	Summary:   "drople removes any values less than or equal to threshold from each series in s. Will error if this operation results in an empty series.",
	Arguments: dropArgs,
	Return:    models.TypeSeriesSet,
}

func DropLe(e *State, T miniprofiler.Timer, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value <= threshold }
	return dropValues(e, T, series, threshold, dropFunction)
}

var dropLDoc = &doc.Func{
	Name:      "dropl",
	Summary:   "dropl removes any values less than threshold from each series in s. Will error if this operation results in an empty series.",
	Arguments: dropArgs,
	Return:    models.TypeSeriesSet,
}

func DropL(e *State, T miniprofiler.Timer, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value < threshold }
	return dropValues(e, T, series, threshold, dropFunction)
}

var dropNADoc = &doc.Func{
	Name:      "dropna",
	Summary:   "dropna removes any NaN or Inf values from each series in s. Will error if this operation results in an empty series.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeSeriesSet,
}

func DropNA(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool {
		return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
	}
	return dropValues(e, T, series, fromScalar(0), dropFunction)
}

var dropBoolDoc = &doc.Func{
	Name:    "dropbool",
	Summary: "dropbool drops datapoints from s where the corresponding value in the condition set is non-zero. (See Series Operations for what corresponding means).",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "condition",
			Type: models.TypeSeriesSet,
		},
	},
	Return:   models.TypeSeriesSet,
	Examples: []doc.HTMLString{doc.HTMLString(dropBoolExampleOne)},
}

func DropBool(e *State, T miniprofiler.Timer, target *Results, filter *Results) (*Results, error) {
	res := Results{}
	unions := e.union(target, filter, "dropbool union")
	for _, union := range unions {
		aSeries := union.A.Value().(Series)
		bSeries := union.B.Value().(Series)
		newSeries := make(Series)
		for k, v := range aSeries {
			if bv, ok := bSeries[k]; ok {
				if bv != float64(0) {
					newSeries[k] = v
				}
			}
		}
		if len(newSeries) > 0 {
			res.Results = append(res.Results, &Result{Group: union.Group, Value: newSeries})
		}
	}
	return &res, nil
}

var filterDoc = &doc.Func{
	Name:    "filter",
	Summary: "filter returns all results in s that are a subset of n and have a non-zero value. Useful with the limit and sort functions to return the top X results of a query.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		nNumberSetArg,
	},
	Return: models.TypeSeriesSet,
}

func Filter(e *State, T miniprofiler.Timer, series *Results, number *Results) (*Results, error) {
	var ns ResultSlice
	for _, sr := range series.Results {
		for _, nr := range number.Results {
			if sr.Group.Subset(nr.Group) || nr.Group.Subset(sr.Group) {
				if nr.Value.Value().(Number) != 0 {
					ns = append(ns, sr)
				}
			}
		}
	}
	series.Results = ns
	return series, nil
}

var limitDoc = &doc.Func{
	Name:    "limit",
	Summary: "limit returns the first count results (set items) from s.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		doc.Arg{
			Name: "count",
			Type: models.TypeScalar,
		},
	},
	Return: models.TypeNumberSet,
}

func Limit(e *State, T miniprofiler.Timer, series *Results, v float64) (*Results, error) {
	i := int(v)
	if len(series.Results) > i {
		series.Results = series.Results[:i]
	}
	return series, nil
}

var tailDoc = &doc.Func{
	Name:    "tail",
	Summary: "tail shortens the number of datapoints for each series in s to length n. If the series is shorter than the number of requested points the series is unchanged as all points are in the requested window.",
	Arguments: doc.Arguments{
		sSeriesSetArg,
		nNumberSetArg,
	},
	Return: models.TypeSeriesSet,
}

func Tail(e *State, T miniprofiler.Timer, series *Results, number *Results) (*Results, error) {
	f := func(res *Results, s *Result, floats []float64) error {
		tailLength := int(floats[0])

		// if there are fewer points than the requested tail
		// short circut and just return current series
		if len(s.Value.Value().(Series)) <= tailLength {
			res.Results = append(res.Results, s)
			return nil
		}

		// create new sorted series
		// not going to do quick select
		// see https://github.com/bosun-monitor/bosun/pull/1802
		// for details
		oldSr := s.Value.Value().(Series)
		sorted := NewSortedSeries(oldSr)

		// create new series keep a reference
		// and point sr.Value interface at reference
		// as we don't need old series any more
		newSeries := make(Series)
		s.Value = newSeries

		// load up new series with desired
		// number of points
		// we already checked len so this is safe
		for _, item := range sorted[len(sorted)-tailLength:] {
			newSeries[item.T] = item.V
		}
		res.Results = append(res.Results, s)
		return nil
	}

	return match(f, series, number)
}

var cropExampleOne = `<pre><code>lookup test {
    entry host=ny-bosun01 {
        start = 30
    }
    entry host=* {
        start = 60
    }
}

alert test {
    template = test
    $q = q("avg:rate:os.cpu{host=ny-bosun*}", "5m", "")
    $c = crop($q, lookup("test", "start") , 0)
    crit = avg($c)
}
</code></pre>`

var dropBoolExampleOne = `<p>Drop tr_avg (avg response time per bucket) datapoints if the count in that bucket was + or - 100 from the average count over the time period.</p>
<pre><code>
$count = q("sum:traffic.haproxy.route_tr_count{host=literal_or(ny-logsql01),route=Questions/Show}", "30m", "")
$avg = q("sum:traffic.haproxy.route_tr_avg{host=literal_or(ny-logsql01),route=Questions/Show}", "30m", "")
$avgCount = avg($count)
dropbool($avg, !($count < $avgCount-100 || $count > $avgCount+100))
</code></pre>`
