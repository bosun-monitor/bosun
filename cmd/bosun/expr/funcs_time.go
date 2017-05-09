package expr

import (
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
)

var timeFuncs = parse.FuncMap{
	"d": {
		Args:   dDoc.Arguments.TypeSlice(),
		Return: dDoc.Return,
		F:      Duration,
		Doc:    dDoc,
	},
	"epoch": {
		Args:   []models.FuncType{},
		Return: models.TypeScalar,
		F:      Epoch,
	},
	"month": {
		Args:   []models.FuncType{models.TypeScalar, models.TypeString},
		Return: models.TypeScalar,
		F:      Month,
	},
	"shift": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Shift,
		Doc:    shiftdoc,
	},
	"timedelta": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      TimeDelta,
	},
	"tod": {
		Args:   []models.FuncType{models.TypeScalar},
		Return: models.TypeString,
		F:      ToDuration,
	},
}

var dDoc = &doc.Func{
	Name:    "d",
	Summary: `d Returns the number of seconds of the <a href="http://opentsdb.net/docs/build/html/user_guide/query/dates.html">OpenTSDB duration string</a> s.`,
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

var shiftdoc = &doc.Func{
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
