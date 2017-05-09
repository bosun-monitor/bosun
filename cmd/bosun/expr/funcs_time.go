package expr

import (
	"fmt"
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/jinzhu/now"
)

var timeFuncs = parse.FuncMap{
	"d": {
		Args:   dDoc.Arguments.TypeSlice(),
		Return: dDoc.Return,
		F:      Duration,
		Doc:    dDoc,
	},
	"epoch": {
		Args:   epochDoc.Arguments.TypeSlice(),
		Return: epochDoc.Return,
		F:      Epoch,
		Doc:    epochDoc,
	},
	"month": {
		Args:   monthDoc.Arguments.TypeSlice(),
		Return: monthDoc.Return,
		F:      Month,
		Doc:    monthDoc,
	},
	"shift": {
		Args:   shiftDoc.Arguments.TypeSlice(),
		Return: shiftDoc.Return,
		Tags:   tagFirst,
		F:      Shift,
		Doc:    shiftDoc,
	},
	"timedelta": {
		Args:   timeDeltaDoc.Arguments.TypeSlice(),
		Return: timeDeltaDoc.Return,
		Tags:   tagFirst,
		F:      TimeDelta,
		Doc:    timeDeltaDoc,
	},
	"tod": {
		Args:   todDoc.Arguments.TypeSlice(),
		Return: todDoc.Return,
		F:      ToDuration,
		Doc:    todDoc,
	},
}

var dDoc = &doc.Func{
	Name:    "d",
	Summary: `d Returns the number of seconds of the <a href="http://opentsdb.net/docs/build/html/user_guide/query/dates.html">OpenTSDB duration string</a> s. This is the inverse of the <code>tod()</code> function`,
	Arguments: doc.Arguments{
		doc.Arg{
			Name: "s",
			Type: models.TypeString,
		},
	},
	Return: models.TypeScalar,
}

func Duration(e *State, T miniprofiler.Timer, d string) (*Results, error) {
	duration, err := opentsdb.ParseDuration(d)
	if err != nil {
		return nil, err
	}
	return &Results{
		Results: []*Result{
			{Value: Scalar(duration.Seconds())},
		},
	}, nil
}

var epochDoc = &doc.Func{
	Name:    "epoch",
	Summary: "epoch returns the Unix epoch in seconds.",
	Return:  models.TypeScalar,
}

func Epoch(e *State, T miniprofiler.Timer) (*Results, error) {
	return &Results{
		Results: []*Result{
			{Value: Scalar(float64(e.now.Unix()))},
		},
	}, nil
}

var monthDoc = &doc.Func{
	Name:    "month",
	Summary: "Returns the epoch of either the start or end of the month. The epoch value will be UTC. Useful for things like monthly billing",
	Arguments: doc.Arguments{
		doc.Arg{
			Name: "offset",
			Desc: "the timezone offset from UTC that the month starts/ends at.",
			Type: models.TypeScalar,
		},
		doc.Arg{
			Name: "startEnd",
			Desc: `must be either "start" or "end"`,
			Type: models.TypeString,
		},
	},
	Return:   models.TypeScalar,
	Examples: []doc.HTMLString{doc.HTMLString(monthExampleOne)},
}

func Month(e *State, T miniprofiler.Timer, offset float64, startEnd string) (*Results, error) {
	if startEnd != "start" && startEnd != "end" {
		return nil, fmt.Errorf("last parameter for mtod must be 'start' or 'end'")
	}
	offsetInt := int(offset)
	location := time.FixedZone(fmt.Sprintf("%v", offsetInt), offsetInt*60*60)
	timeZoned := e.now.In(location)
	var mtod float64
	if startEnd == "start" {
		mtod = float64(now.New(timeZoned).BeginningOfMonth().Unix())
	} else {
		mtod = float64(now.New(timeZoned).EndOfMonth().Unix())
	}
	return &Results{
		Results: []*Result{
			{Value: Scalar(float64(mtod))},
		},
	}, nil
}

var shiftDoc = &doc.Func{
	Name:    "shift",
	Summary: `Shift changes the timestamp of each datapoint in s to be the specified duration in the future. It also adds a tag representing the shift duration. This is meant so you can overlay times visually in a graph.`,
	Arguments: doc.Arguments{
		doc.Arg{
			Name: "s",
			Type: models.TypeSeriesSet,
		},
		doc.Arg{
			Name: "dur",
			Type: models.TypeString,
			Desc: `The amount of time to shift the time series forward by. It is a <a href="http://opentsdb.net/docs/build/html/user_guide/query/dates.html">OpenTSDB duration string</a>. This will be added as a tag to each item in the series. The tag key is "shift" and the value will be this argument.`,
		},
	},
	Return: models.TypeNumberSet,
}

func Shift(e *State, T miniprofiler.Timer, series *Results, d string) (*Results, error) {
	dur, err := opentsdb.ParseDuration(d)
	if err != nil {
		return series, err
	}
	for _, result := range series.Results {
		newSeries := make(Series)
		for t, v := range result.Value.Value().(Series) {
			newSeries[t.Add(time.Duration(dur))] = v
		}
		result.Group["shift"] = d
		result.Value = newSeries
	}
	return series, nil
}

var timeDeltaDoc = &doc.Func{
	Name:      "timedelta",
	Summary:   "timedelta creates a new series where the values are the difference between successive timestamps as seconds for each series in s.",
	Arguments: doc.Arguments{sSeriesSetArg},
	Return:    models.TypeSeriesSet,
	Examples:  []doc.HTMLString{doc.HTMLString(timeDeltaExampleOne)},
}

func TimeDelta(e *State, T miniprofiler.Timer, series *Results) (*Results, error) {
	for _, res := range series.Results {
		sorted := NewSortedSeries(res.Value.Value().(Series))
		newSeries := make(Series)
		if len(sorted) < 2 {
			newSeries[sorted[0].T] = 0
			res.Value = newSeries
			continue
		}
		lastTime := sorted[0].T.Unix()
		for _, dp := range sorted[1:] {
			unixTime := dp.T.Unix()
			diff := unixTime - lastTime
			newSeries[dp.T] = float64(diff)
			lastTime = unixTime
		}
		res.Value = newSeries
	}
	return series, nil
}

var todDoc = &doc.Func{
	Name:    "tod",
	Summary: `tod returns a <a href="http://opentsdb.net/docs/build/html/user_guide/query/dates.html">OpenTSDB duration string</a> that represents the number of seconds in sec. This lets you do math on durations and then pass it to the duration arguments in functions like <code>q()</code>. This is the inverse of the <code>d()</code> function.`,
	Arguments: doc.Arguments{
		doc.Arg{
			Name: "sec",
			Type: models.TypeScalar,
		},
	},
	Return: models.TypeString,
}

func ToDuration(e *State, T miniprofiler.Timer, sec float64) (*Results, error) {
	d := opentsdb.Duration(time.Duration(int64(sec)) * time.Second)
	return &Results{
		Results: []*Result{
			{Value: String(d.HumanString())},
		},
	}, nil
}

var monthExampleOne = `<pre><code>$hostInt = host=ny-nexus01,iname=Ethernet1/46
$inMetric = "sum:5m-avg:rate{counter,,1}:__ny-nexus01.os.net.bytes{$hostInt,direction=in}"
$outMetric = "sum:5m-avg:rate{counter,,1}:__ny-nexus01.os.net.bytes{$hostInt,direction=in}"
$commit = 100
$monthStart = month(-4, "start")
$monthEnd = month(-4, "end")
$monthLength = $monthEnd - $monthStart
$burstTime = ($monthLength)*.05
$burstableObservations = $burstTime / d("5m")
$in = q($inMetric, tod(epoch()-$monthStart), "") * 8 / 1e6
$out = q($inMetric, tod(epoch()-$monthStart), "") * 8 / 1e6
$inOverCount = sum($in > $commit)
$outOverCount = sum($out > $commit)
$inOverCount > $burstableObservations || $outOverCount > $burstableObservations
</code></pre>`

var timeDeltaExampleOne = `<pre><code>timedelta(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1))</code></pre>

<p>Will return a seriesSet equal to the what the following expression returns:</p>
<pre><code>series("foo=bar", 1466133610, 10, 1466133710, 100)</code></pre>
`
