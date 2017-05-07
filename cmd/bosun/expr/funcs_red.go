package expr

import (
	"math"
	"sort"
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"github.com/GaryBoone/GoStats/stats"
	"github.com/MiniProfiler/go/miniprofiler"
)

// Reduction functions
var reductionFuncs = parse.FuncMap{
	"avg": {
		Args:   avgDoc.Arguments.TypeSlice(),
		Return: avgDoc.Return,
		Tags:   tagFirst,
		F:      Avg,
		Doc:    avgDoc,
	},
	"cCount": {
		Args:   cCountDoc.Arguments.TypeSlice(),
		Return: cCountDoc.Return,
		Tags:   tagFirst,
		F:      CCount,
		Doc:    cCountDoc,
	},
	"dev": {
		Args:   devDoc.Arguments.TypeSlice(),
		Return: devDoc.Return,
		Tags:   tagFirst,
		F:      Dev,
		Doc:    devDoc,
	},
	"diff": {
		Args:   diffDoc.Arguments.TypeSlice(),
		Return: diffDoc.Return,
		Tags:   tagFirst,
		F:      Diff,
		Doc:    diffDoc,
	},
	"first": {
		Args:   firstDoc.Arguments.TypeSlice(),
		Return: firstDoc.Return,
		Tags:   tagFirst,
		F:      First,
		Doc:    firstDoc,
	},
	"forecastlr": {
		Args:   forecastlrDoc.Arguments.TypeSlice(),
		Return: forecastlrDoc.Return,
		Tags:   tagFirst,
		F:      Forecast_lr,
		Doc:    forecastlrDoc,
	},
	"last": {
		Args:   lastDoc.Arguments.TypeSlice(),
		Return: lastDoc.Return,
		Tags:   tagFirst,
		F:      Last,
		Doc:    lastDoc,
	},
	"len": {
		Args:   lenDoc.Arguments.TypeSlice(),
		Return: lenDoc.Return,
		Tags:   tagFirst,
		F:      Length,
		Doc:    lenDoc,
	},
	"max": {
		Args:   maxDoc.Arguments.TypeSlice(),
		Return: maxDoc.Return,
		Tags:   tagFirst,
		F:      Max,
		Doc:    maxDoc,
	},
	"median": {
		Args:   medianDoc.Arguments.TypeSlice(),
		Return: medianDoc.Return,
		Tags:   tagFirst,
		F:      Median,
		Doc:    medianDoc,
	},
	"min": {
		Args:   minDoc.Arguments.TypeSlice(),
		Return: minDoc.Return,
		Tags:   tagFirst,
		F:      Min,
		Doc:    minDoc,
	},
	"percentile": {
		Args:   percentileDoc.Arguments.TypeSlice(),
		Return: percentileDoc.Return,
		Tags:   tagFirst,
		F:      Percentile,
		Doc:    percentileDoc,
	},
	"since": {
		Args:   sinceDoc.Arguments.TypeSlice(),
		Return: sinceDoc.Return,
		Tags:   tagFirst,
		F:      Since,
		Doc:    sinceDoc,
	},
	"sum": {
		Args:   sumDoc.Arguments.TypeSlice(),
		Return: sumDoc.Return,
		Tags:   tagFirst,
		F:      Sum,
		Doc:    sumDoc,
	},
	"streak": {
		Args:   streakDoc.Arguments.TypeSlice(),
		Return: streakDoc.Return,
		Tags:   tagFirst,
		F:      Streak,
		Doc:    streakDoc,
	},
}

func reduce(e *State, T miniprofiler.Timer, series *Results, F func(Series, ...float64) float64, args ...*Results) (*Results, error) {
	f := func(res *Results, s *Result, floats []float64) error {
		t := s.Value.(Series)
		if len(t) == 0 {
			return nil
		}
		s.Value = Number(F(t, floats...))
		res.Results = append(res.Results, s)
		return nil
	}
	return match(f, series, args...)
}

var avgDoc = &doc.Func{
	Name:    "avg",
	Summary: "avg returns the arithmetic mean for each series in set s.",
	Arguments: doc.Arguments{
		doc.Arg{
			Name: "s",
			Type: models.TypeSeriesSet,
		},
	},
	Return: models.TypeNumberSet,
}

func Avg(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
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

var cCountDoc = &doc.Func{
	Name:      "cCount",
	Summary:   "cCount returns the change count for each series in the set s. The change count is the number of times in the series a value was not equal to the immediate previous value. Useful for checking if things that should be at a steady value are “flapping”. For example, a series with values [0, 1, 0, 1] would return 3.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func CCount(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, cCount)
}

func cCount(dps Series, args ...float64) (a float64) {
	if len(dps) < 2 {
		return float64(0)
	}
	series := NewSortedSeries(dps)
	count := 0
	last := series[0].V
	for _, p := range series[1:] {
		if p.V != last {
			count++
		}
		last = p.V
	}
	return float64(count)
}

var devDoc = &doc.Func{
	Name:      "dev",
	Summary:   "dev returns the standard deviation for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Dev(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, dev)
}

// dev returns the sample standard deviation of x.
func dev(dps Series, args ...float64) (d float64) {
	if len(dps) == 1 {
		return 0
	}
	a := avg(dps)
	for _, v := range dps {
		d += math.Pow(float64(v)-a, 2)
	}
	d /= float64(len(dps) - 1)
	return math.Sqrt(d)
}

var diffDoc = &doc.Func{
	Name:      "diff",
	Summary:   "diff returns the last value minus the first value for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Diff(e *State, T miniprofiler.Timer, series *Results) (r *Results, err error) {
	return reduce(e, T, series, diff)
}

func diff(dps Series, args ...float64) float64 {
	return last(dps) - first(dps)
}

var firstDoc = &doc.Func{
	Name:      "first",
	Summary:   "first returns the first value (oldest) for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func First(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, first)
}

func first(dps Series, args ...float64) (a float64) {
	var first time.Time
	for k, v := range dps {
		if k.Before(first) || first.IsZero() {
			a = v
			first = k
		}
	}
	return
}

func Forecast_lr(e *State, T miniprofiler.Timer, series *Results, y *Results) (r *Results, err error) {
	return reduce(e, T, series, e.forecast_lr, y)
}

var forecastlrDoc = &doc.Func{
	Name:    "forecastlr",
	Summary: "forecastlr returns the number of seconds until a linear regression of each series in set s will reach y_val.",
	Arguments: doc.Arguments{sSeriesSetArg, doc.Arg{
		Name: "y_val",
		Type: models.TypeNumberSet,
	}},
	Return: models.TypeNumberSet,
}

// forecast_lr returns the number of seconds a linear regression predicts the
// series will take to reach y_val.
func (e *State) forecast_lr(dps Series, args ...float64) float64 {
	const tenYears = time.Hour * 24 * 365 * 10
	yVal := args[0]
	var x []float64
	var y []float64
	for k, v := range dps {
		x = append(x, float64(k.Unix()))
		y = append(y, v)
	}
	var slope, intercept, _, _, _, _ = stats.LinearRegression(x, y)
	it := (yVal - intercept) / slope
	var i64 int64
	if it < math.MinInt64 {
		i64 = math.MinInt64
	} else if it > math.MaxInt64 {
		i64 = math.MaxInt64
	} else if math.IsNaN(it) {
		i64 = e.now.Unix()
	} else {
		i64 = int64(it)
	}
	t := time.Unix(i64, 0)
	s := -e.now.Sub(t)
	if s < -tenYears {
		s = -tenYears
	} else if s > tenYears {
		s = tenYears
	}
	return s.Seconds()
}

var lastDoc = &doc.Func{
	Name:      "last",
	Summary:   "last returns the last value (newest) for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Last(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, last)
}

func last(dps Series, args ...float64) (a float64) {
	var last time.Time
	for k, v := range dps {
		if k.After(last) {
			a = v
			last = k
		}
	}
	return
}

var lenDoc = &doc.Func{
	Name:      "len",
	Summary:   "len returns the the number of values (length) for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Length(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, length)
}

func length(dps Series, args ...float64) (a float64) {
	return float64(len(dps))
}

var maxDoc = &doc.Func{
	Name:      "max",
	Summary:   "max returns the the max value of each series in set s. It is the same as calling <code>percentile(s, 1)</code>.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Max(e *State, T miniprofiler.Timer, series *Results) (r *Results, err error) {
	return reduce(e, T, series, percentile, fromScalar(1))
}

var medianDoc = &doc.Func{
	Name:      "median",
	Summary:   "median returns the median value of each series in set s. It is the same as calling <code>percentile(s, .5)</code>.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Median(e *State, T miniprofiler.Timer, series *Results) (r *Results, err error) {
	return reduce(e, T, series, percentile, fromScalar(.5))
}

var minDoc = &doc.Func{
	Name:      "min",
	Summary:   "min returns the minimum value of each series in set s. It is the same as calling <code>percentile(s, 0)</code>.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Min(e *State, T miniprofiler.Timer, series *Results) (r *Results, err error) {
	return reduce(e, T, series, percentile, fromScalar(0))
}

var percentileDoc = &doc.Func{
	Name:    "percentile",
	Summary: "percentile returns the value at percentile p of each series at in set s. min and max can be simulated using p <= 0 and p >= 1, respectively.",
	Arguments: doc.Arguments{sSeriesSetArg, doc.Arg{
		Name: "p",
		Type: models.TypeNumberSet,
	}},
	Return: models.TypeNumberSet,
}

func Percentile(e *State, T miniprofiler.Timer, series *Results, p *Results) (r *Results, err error) {
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

var sinceDoc = &doc.Func{
	Name:      "since",
	Summary:   "since returns the number of seconds since the most recent datapoint of each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Since(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, e.since)
}

func (e *State) since(dps Series, args ...float64) (a float64) {
	var last time.Time
	for k, v := range dps {
		if k.After(last) {
			a = v
			last = k
		}
	}
	s := e.now.Sub(last)
	return s.Seconds()
}

var streakDoc = &doc.Func{
	Name:      "streak",
	Summary:   "streak returns the length of the longest streak of values that evaluate to true (i.e. max amount of contiguous non-zero values found) for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Streak(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, streak)
}

func streak(dps Series, args ...float64) (a float64) {
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}

	series := NewSortedSeries(dps)

	current := 0
	longest := 0
	for _, p := range series {
		if p.V != 0 {
			current++
		} else {
			longest = max(current, longest)
			current = 0
		}
	}
	longest = max(current, longest)
	return float64(longest)
}

var sumDoc = &doc.Func{
	Name:      "sum",
	Summary:   "sum returns the sum of values for each series in set s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeNumberSet,
}

func Sum(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	return reduce(e, T, series, sum)
}

func sum(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	return
}

var sSeriesSetArg = doc.Arg{
	Name: "s",
	Type: models.TypeSeriesSet,
}
