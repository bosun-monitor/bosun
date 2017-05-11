package expr

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/doc"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/GaryBoone/GoStats/stats"
	"github.com/MiniProfiler/go/miniprofiler"
)

func init() {
	for _, v := range reductionFuncs {
		err := v.Doc.SetCodeLink(v.F)
		if err != nil {
			log.Fatal(err)
		}
	}
	Docs = doc.Docs{
		"reduction": reductionFuncs.DocSlice(),
		"group":     groupFuncs.DocSlice(),
		"time":      timeFuncs.DocSlice(),
		"filter":    filterFuncs.DocSlice(),
		"builtins":  builtins.DocSlice(),
	}
	// b, err := Docs.Wiki()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(b.String())
	// os.Exit(0)

}

var Docs doc.Docs

func tagQuery(args []parse.Node) (parse.Tags, error) {
	n := args[0].(*parse.StringNode)
	// Since all 2.1 queries are valid 2.2 queries, at this time
	// we can just use 2.2 to parse to identify group by tags
	q, err := opentsdb.ParseQuery(n.Text, opentsdb.Version2_2)
	if q == nil && err != nil {
		return nil, err
	}
	t := make(parse.Tags)
	for k := range q.GroupByTags {
		t[k] = struct{}{}
	}
	return t, nil
}

func tagFirst(args []parse.Node) (parse.Tags, error) {
	return args[0].Tags()
}

func seriesFuncTags(args []parse.Node) (parse.Tags, error) {
	t := make(parse.Tags)
	text := args[0].(*parse.StringNode).Text
	if text == "" {
		return t, nil
	}
	ts, err := opentsdb.ParseTags(text)
	if err != nil {
		return nil, err
	}

	for k := range ts {
		t[k] = struct{}{}
	}
	return t, nil
}

var builtins = parse.FuncMap{
	// Other functions

	"abs": {
		Args:   []models.FuncType{models.TypeNumberSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Abs,
	},

	"des": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeScalar, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Des,
	},
	"linelr": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Line_lr,
	},
	"nv": {
		Args:   []models.FuncType{models.TypeNumberSet, models.TypeScalar},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      NV,
	},
	"series": {
		Args:      []models.FuncType{models.TypeString, models.TypeScalar},
		VArgs:     true,
		VArgsPos:  1,
		VArgsOmit: true,
		Return:    models.TypeSeriesSet,
		Tags:      seriesFuncTags,
		F:         SeriesFunc,
	},
	"sort": {
		Args:   []models.FuncType{models.TypeNumberSet, models.TypeString},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Sort,
	},
	"leftjoin": {
		Args:     []models.FuncType{models.TypeString, models.TypeString, models.TypeNumberSet},
		VArgs:    true,
		VArgsPos: 2,
		Return:   models.TypeTable,
		Tags:     nil, // TODO
		F:        LeftJoin,
	},
	"merge": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		VArgs:  true,
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Merge,
	},
	"tail": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Tail,
	},
	"map": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberExpr},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Map,
	},
	"v": {
		Return:  models.TypeScalar,
		F:       V,
		MapFunc: true,
	},
}

func V(e *State, T miniprofiler.Timer) (*Results, error) {
	return fromScalar(e.vValue), nil
}

func Map(e *State, T miniprofiler.Timer, series *Results, expr *Results) (*Results, error) {
	newExpr := Expr{expr.Results[0].Value.Value().(NumberExpr).Tree}
	for _, result := range series.Results {
		newSeries := make(Series)
		for t, v := range result.Value.Value().(Series) {
			e.vValue = v
			subResults, _, err := newExpr.ExecuteState(e, T)
			if err != nil {
				return series, err
			}
			for _, res := range subResults.Results {
				var v float64
				switch res.Value.Value().(type) {
				case Number:
					v = float64(res.Value.Value().(Number))
				case Scalar:
					v = float64(res.Value.Value().(Scalar))
				default:
					return series, fmt.Errorf("wrong return type for map expr: %v", res.Type())
				}
				newSeries[t] = v
			}
		}
		result.Value = newSeries
	}
	return series, nil
}

func SeriesFunc(e *State, T miniprofiler.Timer, tags string, pairs ...float64) (*Results, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("uneven number of time stamps and values")
	}
	group := opentsdb.TagSet{}
	if tags != "" {
		var err error
		group, err = opentsdb.ParseTags(tags)
		if err != nil {
			return nil, fmt.Errorf("unable to parse tags: %v", err)
		}
	}

	series := make(Series)
	for i := 0; i < len(pairs); i += 2 {
		series[time.Unix(int64(pairs[i]), 0)] = pairs[i+1]
	}
	return &Results{
		Results: []*Result{
			{
				Value: series,
				Group: group,
			},
		},
	}, nil
}

func NV(e *State, T miniprofiler.Timer, series *Results, v float64) (results *Results, err error) {
	// If there are no results in the set, promote it to a number with the empty group ({})
	if len(series.Results) == 0 {
		series.Results = append(series.Results, &Result{Value: Number(v), Group: make(opentsdb.TagSet)})
		return series, nil
	}
	series.NaNValue = &v
	return series, nil
}

func Sort(e *State, T miniprofiler.Timer, series *Results, order string) (*Results, error) {
	// Sort by groupname first to make the search deterministic
	sort.Sort(ResultSliceByGroup(series.Results))
	switch order {
	case "desc":
		sort.Stable(sort.Reverse(ResultSliceByValue(series.Results)))
	case "asc":
		sort.Stable(ResultSliceByValue(series.Results))
	default:
		return nil, fmt.Errorf("second argument of order() must be asc or desc")
	}
	return series, nil
}

func Merge(e *State, T miniprofiler.Timer, series ...*Results) (*Results, error) {
	res := &Results{}
	if len(series) == 0 {
		return res, fmt.Errorf("merge requires at least one result")
	}
	if len(series) == 1 {
		return series[0], nil
	}
	seen := make(map[string]bool)
	for _, r := range series {
		for _, entry := range r.Results {
			if _, ok := seen[entry.Group.String()]; ok {
				return res, fmt.Errorf("duplicate group in merge: %s", entry.Group.String())
			}
			seen[entry.Group.String()] = true
		}
		res.Results = append(res.Results, r.Results...)
	}
	return res, nil
}

func LeftJoin(e *State, T miniprofiler.Timer, keysCSV, columnsCSV string, rowData ...*Results) (*Results, error) {
	res := &Results{}
	dataWidth := len(rowData)
	if dataWidth == 0 {
		return res, fmt.Errorf("leftjoin requires at least one item to populate rows")
	}
	keyColumns := strings.Split(keysCSV, ",")
	dataColumns := strings.Split(columnsCSV, ",")
	if len(dataColumns) != dataWidth {
		return res, fmt.Errorf("mismatch in length of data rows and data labels")
	}
	keyWidth := len(keyColumns)
	keyIndex := make(map[string]int, keyWidth)
	for i, v := range keyColumns {
		keyIndex[v] = i
	}
	t := Table{}
	t.Columns = append(keyColumns, dataColumns...)
	rowWidth := len(dataColumns) + len(keyColumns)
	rowGroups := []opentsdb.TagSet{}
	for i, r := range rowData {
		if i == 0 {
			for _, val := range r.Results {
				row := make([]interface{}, rowWidth)
				for k, v := range val.Group {
					if ki, ok := keyIndex[k]; ok {
						row[ki] = v
					}
				}
				row[keyWidth+i] = val.Value.Value()
				rowGroups = append(rowGroups, val.Group)
				t.Rows = append(t.Rows, row)
			}
			continue
		}
		for rowIndex, group := range rowGroups {
			for _, val := range r.Results {
				if group.Subset(val.Group) {
					t.Rows[rowIndex][keyWidth+i] = val.Value.Value()
				}
			}
		}
	}
	return &Results{
		Results: []*Result{
			{Value: t},
		},
	}, nil
}

func fromScalar(f float64) *Results {
	return &Results{
		Results: ResultSlice{
			&Result{
				Value: Number(f),
			},
		},
	}
}

func match(f func(res *Results, series *Result, floats []float64) error, series *Results, numberSets ...*Results) (*Results, error) {
	res := *series
	res.Results = nil
	for _, s := range series.Results {
		var floats []float64
		for _, num := range numberSets {
			for _, n := range num.Results {
				if len(n.Group) == 0 || s.Group.Overlaps(n.Group) {
					floats = append(floats, float64(n.Value.(Number)))
					break
				}
			}
		}
		if len(floats) != len(numberSets) {
			if !series.IgnoreUnjoined {
				return nil, fmt.Errorf("unjoined groups for %s", s.Group)
			}
			continue
		}
		if err := f(&res, s, floats); err != nil {
			return nil, err
		}
	}
	return &res, nil
}

func Abs(e *State, T miniprofiler.Timer, series *Results) *Results {
	for _, s := range series.Results {
		s.Value = Number(math.Abs(float64(s.Value.Value().(Number))))
	}
	return series
}

func Count(e *State, T miniprofiler.Timer, query, sduration, eduration string) (r *Results, err error) {
	r, err = Query(e, T, query, sduration, eduration)
	if err != nil {
		return
	}
	return &Results{
		Results: []*Result{
			{Value: Scalar(len(r.Results))},
		},
	}, nil
}

func Des(e *State, T miniprofiler.Timer, series *Results, alpha float64, beta float64) *Results {
	for _, res := range series.Results {
		sorted := NewSortedSeries(res.Value.Value().(Series))
		if len(sorted) < 2 {
			continue
		}
		des := make(Series)
		s := make([]float64, len(sorted))
		b := make([]float64, len(sorted))
		s[0] = sorted[0].V
		for i := 1; i < len(sorted); i++ {
			s[i] = alpha*sorted[i].V + (1-alpha)*(s[i-1]+b[i-1])
			b[i] = beta*(s[i]-s[i-1]) + (1-beta)*b[i-1]
			des[sorted[i].T] = s[i]
		}
		res.Value = des
	}
	return series
}

func Line_lr(e *State, T miniprofiler.Timer, series *Results, d string) (*Results, error) {
	dur, err := opentsdb.ParseDuration(d)
	if err != nil {
		return series, err
	}
	for _, res := range series.Results {
		res.Value = line_lr(res.Value.(Series), time.Duration(dur))
		res.Group.Merge(opentsdb.TagSet{"regression": "line"})
	}
	return series, nil
}

// line_lr generates a series representing the line up to duration in the future.
func line_lr(dps Series, d time.Duration) Series {
	var x []float64
	var y []float64
	sortedDPS := NewSortedSeries(dps)
	var maxT time.Time
	if len(sortedDPS) > 1 {
		maxT = sortedDPS[len(sortedDPS)-1].T
	}
	for _, v := range sortedDPS {
		xv := float64(v.T.Unix())
		x = append(x, xv)
		y = append(y, v.V)
	}
	var slope, intercept, _, _, _, _ = stats.LinearRegression(x, y)
	s := make(Series)
	// First point in the regression line
	s[maxT] = float64(maxT.Unix())*slope + intercept
	// Last point
	last := maxT.Add(d)
	s[last] = float64(last.Unix())*slope + intercept
	return s
}

var sSeriesSetArg = doc.Arg{
	Name: "s",
	Type: models.TypeSeriesSet,
}

var nNumberSetArg = doc.Arg{
	Name: "n",
	Type: models.TypeNumberSet,
}
