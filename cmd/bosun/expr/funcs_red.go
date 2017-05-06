package expr

import (
	"fmt"
	"os"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"github.com/MiniProfiler/go/miniprofiler"
)

func init() {
	fmt.Println(reductionFuncs["avg"].Doc.Signature())
	os.Exit(0)
}

// Reduction functions
var reductionFuncs = parse.FuncMap{
	"avg": {
		Args:   avgdoc.Arguments.TypeSlice(),
		Return: avgdoc.Return,
		Tags:   tagFirst,
		F:      Avg,
		Doc:    avgdoc,
	},
	"cCount": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      CCount,
	},
	"dev": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Dev,
	},
	"diff": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Diff,
	},
	"first": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      First,
	},
	"forecastlr": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Forecast_lr,
	},
	"linelr": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Line_lr,
	},
	"last": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Last,
	},
	"len": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Length,
	},
	"max": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Max,
	},
	"median": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Median,
	},
	"min": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Min,
	},
	"percentile": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Percentile,
	},
	"since": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Since,
	},
	"sum": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Sum,
	},
	"streak": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Streak,
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

var avgdoc = &doc.Func{
	Name:    "avg",
	Summary: "Returns the arithmetic mean for each series in set s.",
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
