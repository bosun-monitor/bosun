package expr

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/GaryBoone/GoStats/stats"
	"github.com/jinzhu/now"
)

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

func tagRemove(args []parse.Node) (parse.Tags, error) {
	tags, err := tagFirst(args)
	if err != nil {
		return nil, err
	}
	key := args[1].(*parse.StringNode).Text
	delete(tags, key)
	return tags, nil
}

func seriesFuncTags(args []parse.Node) (parse.Tags, error) {
	s := args[0].(*parse.StringNode).Text
	return tagsFromString(s)
}

func aggrFuncTags(args []parse.Node) (parse.Tags, error) {
	if len(args) < 3 {
		return nil, errors.New("aggr: expect 3 arguments")
	}
	if _, ok := args[1].(*parse.StringNode); !ok {
		return nil, errors.New("aggr: expect group to be string")
	}
	s := args[1].(*parse.StringNode).Text
	if s == "" {
		return tagsFromString(s)
	}
	tags := strings.Split(s, ",")
	for i := range tags {
		tags[i] += "=*"
	}
	return tagsFromString(strings.Join(tags, ","))
}

func tagsFromString(text string) (parse.Tags, error) {
	t := make(parse.Tags)
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

func tagTranspose(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	sp := strings.Split(args[1].(*parse.StringNode).Text, ",")
	if sp[0] != "" {
		for _, t := range sp {
			tags[t] = struct{}{}
		}
	}
	if atags, err := args[0].Tags(); err != nil {
		return nil, err
	} else if !tags.Subset(atags) {
		return nil, fmt.Errorf("transpose tags (%v) must be a subset of first argument's tags (%v)", tags, atags)
	}
	return tags, nil
}

func tagRename(args []parse.Node) (parse.Tags, error) {
	tags, err := tagFirst(args)
	if err != nil {
		return nil, err
	}
	for _, section := range strings.Split(args[1].(*parse.StringNode).Text, ",") {
		kv := strings.Split(section, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("error passing groups")
		}
		for oldTagKey := range tags {
			if kv[0] == oldTagKey {
				if _, ok := tags[kv[1]]; ok {
					return nil, fmt.Errorf("%s already in group", kv[1])
				}
				delete(tags, kv[0])
				tags[kv[1]] = struct{}{}
			}
		}
	}
	return tags, nil
}

var builtins = map[string]parse.Func{
	// Reduction functions

	"avg": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeNumberSet,
		Tags:   tagFirst,
		F:      Avg,
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

	// Aggregation functions
	"aggr": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   aggrFuncTags,
		F:      Aggr,
		Check:  aggrCheck,
	},

	// Group functions
	"addtags": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		Tags:          tagRename,
		F:             AddTags,
	},
	"rename": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		Tags:          tagRename,
		F:             Rename,
	},
	"remove": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		Tags:          tagRemove,
		F:             Remove,
	},
	"t": {
		Args:   []models.FuncType{models.TypeNumberSet, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagTranspose,
		F:      Transpose,
	},
	"ungroup": {
		Args:   []models.FuncType{models.TypeNumberSet},
		Return: models.TypeScalar,
		F:      Ungroup,
	},

	// Other functions

	"abs": {
		Args:          []models.FuncType{models.TypeVariantSet},
		VariantReturn: true,
		Tags:          tagFirst,
		F:             Abs,
	},
	"crop": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Crop,
	},
	"d": {
		Args:   []models.FuncType{models.TypeString},
		Return: models.TypeScalar,
		F:      Duration,
	},
	"tod": {
		Args:   []models.FuncType{models.TypeScalar},
		Return: models.TypeString,
		F:      ToDuration,
	},
	"des": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeScalar, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Des,
	},
	"dropge": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropGe,
	},
	"dropg": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropG,
	},
	"drople": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropLe,
	},
	"dropl": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropL,
	},
	"dropna": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropNA,
	},
	"dropbool": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeSeriesSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      DropBool,
	},
	"epoch": {
		Args:   []models.FuncType{},
		Return: models.TypeScalar,
		F:      Epoch,
	},
	"filter": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeNumberSet},
		VariantReturn: true,
		Tags:          tagFirst,
		F:             Filter,
	},
	"limit": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeScalar},
		VariantReturn: true,
		Tags:          tagFirst,
		F:             Limit,
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
	"shift": {
		Args:   []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      Shift,
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
	"month": {
		Args:   []models.FuncType{models.TypeScalar, models.TypeString},
		Return: models.TypeScalar,
		F:      Month,
	},
	"timedelta": {
		Args:   []models.FuncType{models.TypeSeriesSet},
		Return: models.TypeSeriesSet,
		Tags:   tagFirst,
		F:      TimeDelta,
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

// Aggr combines multiple series matching the specified groups using an aggregator function. If group
// is empty, all given series are combined, regardless of existing groups.
// Available aggregator functions include: avg, min, max, sum, and pN, where N is a float between
// 0 and 1 inclusive, e.g. p.50 represents the 50th percentile. p0 and p1 are equal to min and max,
// respectively, but min and max are preferred for readability.
func Aggr(e *State, series *Results, groups string, aggregator string) (*Results, error) {
	results := Results{}

	grps := splitGroups(groups)
	if len(grps) == 0 {
		// no groups specified, so we merge all group values
		res, err := aggr(e, series, aggregator)
		if err != nil {
			return &results, err
		}
		res.Group = opentsdb.TagSet{}
		results.Results = append(results.Results, res)
		return &results, nil
	}

	// at least one group specified, so we work out what
	// the new group values will be
	newGroups := map[string]*Results{}
	for _, result := range series.Results {
		var vals []string
		for _, grp := range grps {
			if val, ok := result.Group[grp]; ok {
				vals = append(vals, val)
				continue
			}
			return nil, fmt.Errorf("unmatched group in at least one series: %v", grp)
		}
		groupName := strings.Join(vals, ",")
		if _, ok := newGroups[groupName]; !ok {
			newGroups[groupName] = &Results{}
		}
		newGroups[groupName].Results = append(newGroups[groupName].Results, result)
	}

	for groupName, series := range newGroups {
		res, err := aggr(e, series, aggregator)
		if err != nil {
			return &results, err
		}
		vs := strings.Split(groupName, ",")
		res.Group = opentsdb.TagSet{}
		for i := 0; i < len(grps); i++ {
			res.Group.Merge(opentsdb.TagSet{grps[i]: vs[i]})
		}
		results.Results = append(results.Results, res)
	}

	return &results, nil
}

// Splits a string of groups by comma, but also trims any added whitespace
// and returns an empty slice if the string is empty.
func splitGroups(groups string) []string {
	if len(groups) == 0 {
		return []string{}
	}
	grps := strings.Split(groups, ",")
	for i, grp := range grps {
		grps[i] = strings.Trim(grp, " ")
	}
	return grps
}

func aggr(e *State, series *Results, aggfunc string) (*Result, error) {
	res := Result{}
	newSeries := make(Series)
	var isPerc bool
	var percValue float64
	if len(aggfunc) > 0 && aggfunc[0] == 'p' {
		var err error
		percValue, err = strconv.ParseFloat(aggfunc[1:], 10)
		isPerc = err == nil
	}
	if isPerc {
		if percValue < 0 || percValue > 1 {
			return nil, fmt.Errorf("expr: aggr: percentile number must be greater than or equal to zero 0 and less than or equal 1")
		}
		aggfunc = "percentile"
	}

	switch aggfunc {
	case "percentile":
		newSeries = aggrPercentile(series.Results, percValue)
	case "min":
		newSeries = aggrPercentile(series.Results, 0.0)
	case "max":
		newSeries = aggrPercentile(series.Results, 1.0)
	case "avg":
		newSeries = aggrAverage(series.Results)
	case "sum":
		newSeries = aggrSum(series.Results)
	default:
		return &res, fmt.Errorf("unknown aggfunc: %v. Options are avg, p50, min, max", aggfunc)
	}

	res.Value = newSeries
	return &res, nil
}

func aggrPercentile(series ResultSlice, percValue float64) Series {
	newSeries := make(Series)
	merged := map[time.Time][]float64{}
	for _, result := range series {
		for t, v := range result.Value.Value().(Series) {
			merged[t] = append(merged[t], v)
		}
	}
	for t := range merged {
		// transform points from merged series into a made-up
		// single timeseries, so that we can use the existing
		// percentile reduction function here
		dps := Series{}
		for i := range merged[t] {
			dps[time.Unix(int64(i), 0)] = merged[t][i]
		}
		newSeries[t] = percentile(dps, percValue)
	}
	return newSeries
}

func aggrAverage(series ResultSlice) Series {
	newSeries := make(Series)
	counts := map[time.Time]int64{}
	for _, result := range series {
		for t, v := range result.Value.Value().(Series) {
			newSeries[t] += v
			counts[t]++
		}
	}
	for t := range newSeries {
		newSeries[t] /= float64(counts[t])
	}
	return newSeries
}

func aggrSum(series ResultSlice) Series {
	newSeries := make(Series)
	for _, result := range series {
		for t, v := range result.Value.Value().(Series) {
			newSeries[t] += v
		}
	}
	return newSeries
}

func aggrCheck(t *parse.Tree, f *parse.FuncNode) error {
	if len(f.Args) < 3 {
		return errors.New("aggr: expect 3 arguments")
	}
	if _, ok := f.Args[2].(*parse.StringNode); !ok {
		return errors.New("aggr: expect string as aggregator function name")
	}
	name := f.Args[2].(*parse.StringNode).Text
	var isPerc bool
	var percValue float64
	if len(name) > 0 && name[0] == 'p' {
		var err error
		percValue, err = strconv.ParseFloat(name[1:], 10)
		isPerc = err == nil
	}
	if isPerc {
		if percValue < 0 || percValue > 1 {
			return errors.New("aggr: percentile number must be greater than or equal to zero 0 and less than or equal 1")
		}
		return nil
	}
	switch name {
	case "avg", "min", "max", "sum":
		return nil
	}
	return fmt.Errorf("aggr: unrecognized aggregation function %s", name)
}

func V(e *State) (*Results, error) {
	return fromScalar(e.vValue), nil
}

func Map(e *State, series *Results, expr *Results) (*Results, error) {
	newExpr := Expr{expr.Results[0].Value.Value().(NumberExpr).Tree}
	for _, result := range series.Results {
		newSeries := make(Series)
		for t, v := range result.Value.Value().(Series) {
			e.vValue = v
			subResults, _, err := newExpr.ExecuteState(e)
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

func SeriesFunc(e *State, tags string, pairs ...float64) (*Results, error) {
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

func Crop(e *State, sSet *Results, startSet *Results, endSet *Results) (*Results, error) {
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

func DropBool(e *State, target *Results, filter *Results) (*Results, error) {
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

func Epoch(e *State) (*Results, error) {
	return &Results{
		Results: []*Result{
			{Value: Scalar(float64(e.now.Unix()))},
		},
	}, nil
}

func Month(e *State, offset float64, startEnd string) (*Results, error) {
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

func NV(e *State, series *Results, v float64) (results *Results, err error) {
	// If there are no results in the set, promote it to a number with the empty group ({})
	if len(series.Results) == 0 {
		series.Results = append(series.Results, &Result{Value: Number(v), Group: make(opentsdb.TagSet)})
		return series, nil
	}
	series.NaNValue = &v
	return series, nil
}

func Sort(e *State, series *Results, order string) (*Results, error) {
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

func Limit(e *State, set *Results, v float64) (*Results, error) {
	if v < 0 {
		return nil, errors.New(fmt.Sprintf("Limit can't be negative value. We have received value %f as limit", v))
	}
	i := int(v)
	if len(set.Results) > i {
		set.Results = set.Results[:i]
	}
	return set, nil
}

func Filter(e *State, set *Results, numberSet *Results) (*Results, error) {
	var ns ResultSlice
	for _, sr := range set.Results {
		for _, nr := range numberSet.Results {
			if sr.Group.Subset(nr.Group) || nr.Group.Subset(sr.Group) {
				if nr.Value.Value().(Number) != 0 {
					ns = append(ns, sr)
				}
			}
		}
	}
	set.Results = ns
	return set, nil
}

func Tail(e *State, series *Results, number *Results) (*Results, error) {
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

func Merge(e *State, series ...*Results) (*Results, error) {
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

func Remove(e *State, set *Results, tagKey string) (*Results, error) {
	seen := make(map[string]bool)
	for _, r := range set.Results {
		if _, ok := r.Group[tagKey]; ok {
			delete(r.Group, tagKey)
			if _, ok := seen[r.Group.String()]; ok {
				return set, fmt.Errorf("duplicate group would result from removing tag key: %v", tagKey)
			}
			seen[r.Group.String()] = true
		} else {
			return set, fmt.Errorf("tag key %v not found in result", tagKey)
		}
	}
	return set, nil
}

func LeftJoin(e *State, keysCSV, columnsCSV string, rowData ...*Results) (*Results, error) {
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

func Shift(e *State, series *Results, d string) (*Results, error) {
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

func Duration(e *State, d string) (*Results, error) {
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

func ToDuration(e *State, sec float64) (*Results, error) {
	d := opentsdb.Duration(time.Duration(int64(sec)) * time.Second)
	return &Results{
		Results: []*Result{
			{Value: String(d.HumanString())},
		},
	}, nil
}

func DropValues(e *State, series *Results, threshold *Results, dropFunction func(float64, float64) bool) (*Results, error) {
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

func DropGe(e *State, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value >= threshold }
	return DropValues(e, series, threshold, dropFunction)
}

func DropG(e *State, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value > threshold }
	return DropValues(e, series, threshold, dropFunction)
}

func DropLe(e *State, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value <= threshold }
	return DropValues(e, series, threshold, dropFunction)
}

func DropL(e *State, series *Results, threshold *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool { return value < threshold }
	return DropValues(e, series, threshold, dropFunction)
}

func DropNA(e *State, series *Results) (*Results, error) {
	dropFunction := func(value float64, threshold float64) bool {
		return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
	}
	return DropValues(e, series, fromScalar(0), dropFunction)
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

func reduce(e *State, series *Results, F func(Series, ...float64) float64, args ...*Results) (*Results, error) {
	f := func(res *Results, s *Result, floats []float64) error {
		switch tp := s.Value.(type) {
		case Series:
			t := s.Value.(Series)
			if len(t) == 0 {
				return nil
			}
			s.Value = Number(F(t, floats...))
			res.Results = append(res.Results, s)
			return nil
		default:
			return errors.New(
				fmt.Sprintf(
					"Unsupported type passed to reduce for alarm [%s]. Want: Series, got: %s. "+
						"It can happen when we can't unjoin values. Please set IgnoreUnjoined and/or "+
						"IgnoreOtherUnjoined for distiguish this error.", e.Origin, reflect.TypeOf(tp).String(),
				),
			)
		}

	}
	return match(f, series, args...)
}

func Abs(e *State, set *Results) *Results {
	for _, s := range set.Results {
		switch s.Type() {
		case models.TypeNumberSet:
			s.Value = Number(math.Abs(float64(s.Value.Value().(Number))))
		case models.TypeSeriesSet:
			for k, v := range s.Value.Value().(Series) {
				s.Value.Value().(Series)[k] = math.Abs(v)
			}
		}
	}
	return set
}

func Diff(e *State, series *Results) (r *Results, err error) {
	return reduce(e, series, diff)
}

func diff(dps Series, args ...float64) float64 {
	return last(dps) - first(dps)
}

func Avg(e *State, series *Results) (*Results, error) {
	return reduce(e, series, avg)
}

// avg returns the mean of x.
func avg(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

func CCount(e *State, series *Results) (*Results, error) {
	return reduce(e, series, cCount)
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

func TimeDelta(e *State, series *Results) (*Results, error) {
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

func Count(e *State, query, sduration, eduration string) (r *Results, err error) {
	r, err = Query(e, query, sduration, eduration)
	if err != nil {
		return
	}
	return &Results{
		Results: []*Result{
			{Value: Scalar(len(r.Results))},
		},
	}, nil
}

func Sum(e *State, series *Results) (*Results, error) {
	return reduce(e, series, sum)
}

func sum(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	return
}

func Des(e *State, series *Results, alpha float64, beta float64) *Results {
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

func Streak(e *State, series *Results) (*Results, error) {
	return reduce(e, series, streak)
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

func Dev(e *State, series *Results) (*Results, error) {
	return reduce(e, series, dev)
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

func Length(e *State, series *Results) (*Results, error) {
	return reduce(e, series, length)
}

func length(dps Series, args ...float64) (a float64) {
	return float64(len(dps))
}

func Last(e *State, series *Results) (*Results, error) {
	return reduce(e, series, last)
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

func First(e *State, series *Results) (*Results, error) {
	return reduce(e, series, first)
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

func Since(e *State, series *Results) (*Results, error) {
	return reduce(e, series, e.since)
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

func Forecast_lr(e *State, series *Results, y *Results) (r *Results, err error) {
	return reduce(e, series, e.forecast_lr, y)
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

func Line_lr(e *State, series *Results, d string) (*Results, error) {
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

func Percentile(e *State, series *Results, p *Results) (r *Results, err error) {
	return reduce(e, series, percentile, p)
}

func Min(e *State, series *Results) (r *Results, err error) {
	return reduce(e, series, percentile, fromScalar(0))
}

func Median(e *State, series *Results) (r *Results, err error) {
	return reduce(e, series, percentile, fromScalar(.5))
}

func Max(e *State, series *Results) (r *Results, err error) {
	return reduce(e, series, percentile, fromScalar(1))
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

func Rename(e *State, set *Results, s string) (*Results, error) {
	for _, section := range strings.Split(s, ",") {
		kv := strings.Split(section, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("error passing groups")
		}
		oldKey, newKey := kv[0], kv[1]
		for _, res := range set.Results {
			for tag, v := range res.Group {
				if oldKey == tag {
					if _, ok := res.Group[newKey]; ok {
						return nil, fmt.Errorf("%s already in group", newKey)
					}
					delete(res.Group, oldKey)
					res.Group[newKey] = v
				}

			}
		}
	}
	return set, nil
}

func AddTags(e *State, set *Results, s string) (*Results, error) {
	if s == "" {
		return set, nil
	}
	tagSetToAdd, err := opentsdb.ParseTags(s)
	if err != nil {
		return nil, err
	}
	for tagKey, tagValue := range tagSetToAdd {
		for _, res := range set.Results {
			if res.Group == nil {
				res.Group = make(opentsdb.TagSet)
			}
			if _, ok := res.Group[tagKey]; ok {
				return nil, fmt.Errorf("%s key already in group", tagKey)
			}
			res.Group[tagKey] = tagValue
		}
	}
	return set, nil
}

func Ungroup(e *State, d *Results) (*Results, error) {
	if len(d.Results) != 1 {
		return nil, fmt.Errorf("ungroup: requires exactly one group")
	}
	return &Results{
		Results: ResultSlice{
			&Result{
				Value: Scalar(d.Results[0].Value.Value().(Number)),
			},
		},
	}, nil
}

func Transpose(e *State, d *Results, gp string) (*Results, error) {
	gps := strings.Split(gp, ",")
	m := make(map[string]*Result)
	for _, v := range d.Results {
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
			i := int64(len(r.Value.(Series)))
			r.Value.(Series)[time.Unix(i, 0).UTC()] = float64(t)
			r.Computations = append(r.Computations, v.Computations...)
		default:
			panic(fmt.Errorf("expr: expected a number"))
		}
	}
	var r Results
	for _, res := range m {
		r.Results = append(r.Results, res)
	}
	return &r, nil
}
