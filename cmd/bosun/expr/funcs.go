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

// TagFirst uses the Tags method on the first argument of a function. The first argument
// should be an already parsed node in order to identify the tags for the first node.
// This function is commonly used to extract the expects tags of functions where the tag
// keys will be the same as the first argument's object.
func TagFirst(args []parse.Node) (parse.TagKeys, error) {
	return args[0].Tags()
}

// tagRemove extracts the expected resulting tags keys from a call to the Remove function.
// It determines that tags by looking at the tags of the first node, and and removing the specified key of
// the second argument from the tagset
func tagRemove(args []parse.Node) (parse.TagKeys, error) {
	tags, err := TagFirst(args)
	if err != nil {
		return nil, err
	}
	key := args[1].(*parse.StringNode).Text
	delete(tags, key)
	return tags, nil
}

// seriesFuncTag extracts the expected resulting tags keys from a call to the SeriesFunc.
// it extracts the keys from the opentsdb style tags/value pairs of the first argument.
func seriesFuncTags(args []parse.Node) (parse.TagKeys, error) {
	s := args[0].(*parse.StringNode).Text
	return tagsFromString(s)
}

// aggrFuncTags extracts the expected resulting tags keys from a call to the Aggr func
func aggrFuncTags(args []parse.Node) (parse.TagKeys, error) {
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

// tagsFromString parse opentsdb style tags from the text and returns
// the tag keys.
func tagsFromString(text string) (parse.TagKeys, error) {
	t := make(parse.TagKeys)
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

// tagTranspose extracts the expected resulting tags keys from a call to the Transpose function.
// It parses the tags from the second CSV string argument and ensures that they are
// a subset of the first arguments expected tags. The returned tags will be based on the
// second argument.
func tagTranspose(args []parse.Node) (parse.TagKeys, error) {
	tags := make(parse.TagKeys)
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

// tagRename extracts the expected resulting tags keys from a call to the Rename function.
// it use the a specification of oldKey=New parsed from the second argument to discover
// the newly named tag keys after processing the keys from the first's arguments Node.
func tagRename(args []parse.Node) (parse.TagKeys, error) {
	tags, err := TagFirst(args)
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

// builtins is a map of the function name in the expression language to the Func specifications
// for all functions available to Bosun even when no datasource is enabled.
// See the documentation for parse.Func to understand the purpose of the various fields.
var builtins = map[string]parse.Func{
	// Reduction functions

	"avg": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Avg,
	},
	"cCount": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       CCount,
	},
	"dev": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Dev,
	},
	"diff": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Diff,
	},
	"first": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       First,
	},
	"forecastlr": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       ForecastLR,
	},
	"linelr": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       LineLR,
	},
	"last": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Last,
	},
	"len": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Length,
	},
	"max": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Max,
	},
	"median": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Median,
	},
	"min": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Min,
	},
	"percentile": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Percentile,
	},
	"since": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Since,
	},
	"sum": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Sum,
	},
	"streak": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Streak,
	},

	// Aggregation functions
	"aggr": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeString, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: aggrFuncTags,
		F:       Aggr,
		Check:   aggrCheck,
	},

	// Group functions
	"addtags": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		TagKeys:       tagRename,
		F:             AddTags,
	},
	"rename": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		TagKeys:       tagRename,
		F:             Rename,
	},
	"remove": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeString},
		VariantReturn: true,
		TagKeys:       tagRemove,
		F:             Remove,
	},
	"t": {
		Args:    []models.FuncType{models.TypeNumberSet, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: tagTranspose,
		F:       Transpose,
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
		TagKeys:       TagFirst,
		F:             Abs,
	},
	"crop": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Crop,
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
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeScalar, models.TypeScalar},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Des,
	},
	"dropge": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropGe,
	},
	"dropg": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropG,
	},
	"drople": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropLe,
	},
	"dropl": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropL,
	},
	"dropna": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropNA,
	},
	"dropbool": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeSeriesSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       DropBool,
	},
	"epoch": {
		Args:   []models.FuncType{},
		Return: models.TypeScalar,
		F:      Epoch,
	},
	"filter": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeNumberSet},
		VariantReturn: true,
		TagKeys:       TagFirst,
		F:             Filter,
	},
	"limit": {
		Args:          []models.FuncType{models.TypeVariantSet, models.TypeScalar},
		VariantReturn: true,
		TagKeys:       TagFirst,
		F:             Limit,
	},
	"isnan": {
		Args:    []models.FuncType{models.TypeNumberSet},
		F:       IsNaN,
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
	},
	"nv": {
		Args:    []models.FuncType{models.TypeNumberSet, models.TypeScalar},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       NV,
	},
	"series": {
		Args:      []models.FuncType{models.TypeString, models.TypeScalar},
		VArgs:     true,
		VArgsPos:  1,
		VArgsOmit: true,
		Return:    models.TypeSeriesSet,
		TagKeys:   seriesFuncTags,
		F:         CreateSeries,
	},
	"sort": {
		Args:    []models.FuncType{models.TypeNumberSet, models.TypeString},
		Return:  models.TypeNumberSet,
		TagKeys: TagFirst,
		F:       Sort,
	},
	"shift": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeString},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Shift,
	},
	"leftjoin": {
		Args:     []models.FuncType{models.TypeString, models.TypeString, models.TypeNumberSet},
		VArgs:    true,
		VArgsPos: 2,
		Return:   models.TypeTable,
		TagKeys:  nil, // TODO
		F:        LeftJoin,
	},
	"merge": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		VArgs:   true,
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Merge,
	},
	"month": {
		Args:   []models.FuncType{models.TypeScalar, models.TypeString},
		Return: models.TypeScalar,
		F:      Month,
	},
	"timedelta": {
		Args:    []models.FuncType{models.TypeSeriesSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       TimeDelta,
	},
	"tail": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberSet},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Tail,
	},
	"map": {
		Args:    []models.FuncType{models.TypeSeriesSet, models.TypeNumberExpr},
		Return:  models.TypeSeriesSet,
		TagKeys: TagFirst,
		F:       Map,
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
func Aggr(e *State, seriesSet *ValueSet, groups string, aggregator string) (*ValueSet, error) {
	results := ValueSet{}

	grps := splitGroups(groups)
	if len(grps) == 0 {
		// no groups specified, so we merge all group values
		res, err := aggr(e, seriesSet, aggregator)
		if err != nil {
			return &results, err
		}
		res.Group = opentsdb.TagSet{}
		results.Append(res)
		return &results, nil
	}

	// at least one group specified, so we work out what
	// the new group values will be
	newGroups := map[string]*ValueSet{}
	for _, result := range seriesSet.Elements {
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
			newGroups[groupName] = &ValueSet{}
		}
		newGroups[groupName].Append(result)
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
		results.Append(res)
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

func aggr(e *State, seriesSet *ValueSet, aggfunc string) (*Element, error) {
	res := Element{}
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
		newSeries = aggrPercentile(seriesSet.Elements, percValue)
	case "min":
		newSeries = aggrPercentile(seriesSet.Elements, 0.0)
	case "max":
		newSeries = aggrPercentile(seriesSet.Elements, 1.0)
	case "avg":
		newSeries = aggrAverage(seriesSet.Elements)
	case "sum":
		newSeries = aggrSum(seriesSet.Elements)
	default:
		return &res, fmt.Errorf("unknown aggfunc: %v. Options are avg, p50, min, max", aggfunc)
	}

	res.Value = newSeries
	return &res, nil
}

func aggrPercentile(series ElementSlice, percValue float64) Series {
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
		newSeries[t] = seriesPercentile(dps, percValue)
	}
	return newSeries
}

func aggrAverage(series ElementSlice) Series {
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

func aggrSum(series ElementSlice) Series {
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

// V is the "v" function in the expression language which is only valid within subExpression (the "map" function).
func V(e *State) (*ValueSet, error) {
	return FromScalar(e.vValue), nil
}

// Map is the "map" function in the expression language.
// It may be removed in the future now that we have variant sets.
func Map(e *State, seriesSet *ValueSet, expr *ValueSet) (*ValueSet, error) {
	newExpr := Expr{expr.Elements[0].Value.Value().(NumberExpr).Tree}
	for _, result := range seriesSet.Elements {
		newSeries := make(Series)
		for t, v := range result.Value.Value().(Series) {
			e.vValue = v
			subResults, _, err := newExpr.ExecuteState(e)
			if err != nil {
				return seriesSet, err
			}
			for _, res := range subResults.Elements {
				var v float64
				switch res.Value.Value().(type) {
				case Number:
					v = float64(res.Value.Value().(Number))
				case Scalar:
					v = float64(res.Value.Value().(Scalar))
				default:
					return seriesSet, fmt.Errorf("wrong return type for map expr: %v", res.Type())
				}
				newSeries[t] = v
			}
		}
		result.Value = newSeries
	}
	return seriesSet, nil
}

// CreateSeries is the "series" function in the expression language.
func CreateSeries(e *State, tags string, pairs ...float64) (*ValueSet, error) {
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
	return &ValueSet{
		Elements: []*Element{
			{
				Value: series,
				Group: group,
			},
		},
	}, nil
}

// Crop is the "crop" function in the expression language.
func Crop(e *State, seriesSet *ValueSet, startSet *ValueSet, endSet *ValueSet) (*ValueSet, error) {
	results := ValueSet{}
INNER:
	for _, seriesResult := range seriesSet.Elements {
		for _, startResult := range startSet.Elements {
			for _, endResult := range endSet.Elements {
				startHasNoGroup := len(startResult.Group) == 0
				endHasNoGroup := len(endResult.Group) == 0
				startOverlapsSeries := seriesResult.Group.Overlaps(startResult.Group)
				endOverlapsSeries := seriesResult.Group.Overlaps(endResult.Group)
				if (startHasNoGroup || startOverlapsSeries) && (endHasNoGroup || endOverlapsSeries) {
					res := cropSeries(e, seriesResult, startResult, endResult)
					results.Append(res)
					continue INNER
				}
			}
		}
	}
	return &results, nil
}

func cropSeries(e *State, seriesResult *Element, startResult *Element, endResult *Element) *Element {
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

// DropBool is the "drop" function in the expression language.
func DropBool(e *State, target *ValueSet, filter *ValueSet) (*ValueSet, error) {
	res := ValueSet{}
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
			res.Append(&Element{Group: union.Group, Value: newSeries})
		}
	}
	return &res, nil
}

// Epoch is the "map" function in the expression language.
func Epoch(e *State) (*ValueSet, error) {
	return &ValueSet{
		Elements: []*Element{
			{Value: Scalar(float64(e.now.Unix()))},
		},
	}, nil
}

// IsNaN is the "isnan" function in the expression language.
func IsNaN(e *State, numberSet *ValueSet) (*ValueSet, error) {
	for _, res := range numberSet.Elements {
		if math.IsNaN(float64(res.Value.Value().(Number))) {
			res.Value = Number(1)
			continue
		}
		res.Value = Number(0)
	}
	return numberSet, nil
}

// Month is the "map" function in the expression language.
func Month(e *State, offset float64, startEnd string) (*ValueSet, error) {
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
	return &ValueSet{
		Elements: []*Element{
			{Value: Scalar(float64(mtod))},
		},
	}, nil
}

// NV is the "nv" function in the expression language.
// If the numberSet is empty (no Results) then a Result is returned with an empty tagset
// and the provided value. Otherwise the numberSet has its NaNValue set which is used to
// replace any NaN values within the seriesSet when it is used in the left and/or
// right of a union operation.
func NV(e *State, numberSet *ValueSet, v float64) (results *ValueSet, err error) {
	// If there are no results in the set, promote it to a number with the empty group ({})
	if len(numberSet.Elements) == 0 {
		numberSet.Append(&Element{Value: Number(v), Group: make(opentsdb.TagSet)})
		return numberSet, nil
	}
	numberSet.NaNValue = &v
	return numberSet, nil
}

// Sort is the "sort" function in the expression language.
func Sort(e *State, numberSet *ValueSet, order string) (*ValueSet, error) {
	// Sort by groupname first to make the search deterministic
	sort.Sort(ElementSliceByGroup(numberSet.Elements))
	switch order {
	case "desc":
		sort.Stable(sort.Reverse(ElementSliceByValue(numberSet.Elements)))
	case "asc":
		sort.Stable(ElementSliceByValue(numberSet.Elements))
	default:
		return nil, fmt.Errorf("second argument of order() must be asc or desc")
	}
	return numberSet, nil
}

// Limit is the "limit" function in the expression language.
func Limit(e *State, set *ValueSet, v float64) (*ValueSet, error) {
	if v < 0 {
		return nil, fmt.Errorf("Limit can't be negative value. We have received value %f as limit", v)
	}
	i := int(v)
	if len(set.Elements) > i {
		set.Elements = set.Elements[:i]
	}
	return set, nil
}

// Filter is the "filter" function in the expression language.
func Filter(e *State, set *ValueSet, numberSet *ValueSet) (*ValueSet, error) {
	var ns ElementSlice
	for _, sr := range set.Elements {
		for _, nr := range numberSet.Elements {
			if sr.Group.Subset(nr.Group) || nr.Group.Subset(sr.Group) {
				if nr.Value.Value().(Number) != 0 {
					ns = append(ns, sr)
				}
			}
		}
	}
	set.Elements = ns
	return set, nil
}

// Tail is the "tail" function in the expression language.
func Tail(e *State, seriesSet *ValueSet, numberSet *ValueSet) (*ValueSet, error) {
	tailSeries := func(res *ValueSet, s *Element, floats []float64) error {
		tailLength := int(floats[0])

		// if there are fewer points than the requested tail
		// short circut and just return current series
		if len(s.Value.Value().(Series)) <= tailLength {
			res.Append(s)
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
		res.Append(s)
		return nil
	}

	return match(tailSeries, seriesSet, numberSet)
}

// Merge is the "merge" function in the expression language.
func Merge(e *State, seriesSets ...*ValueSet) (*ValueSet, error) {
	res := &ValueSet{}
	if len(seriesSets) == 0 {
		return res, fmt.Errorf("merge requires at least one result")
	}
	if len(seriesSets) == 1 {
		return seriesSets[0], nil
	}
	seen := make(map[string]bool)
	for _, sSet := range seriesSets {
		for _, entry := range sSet.Elements {
			if _, ok := seen[entry.Group.String()]; ok {
				return res, fmt.Errorf("duplicate group in merge: %s", entry.Group.String())
			}
			seen[entry.Group.String()] = true
		}
		res.Append(sSet.Elements...)
	}
	return res, nil
}

// Remove is the "remove" function in the expresion language.
func Remove(e *State, set *ValueSet, tagKey string) (*ValueSet, error) {
	seen := make(map[string]bool)
	for _, r := range set.Elements {
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

// LeftJoin is the "leftjoin" function in the expression language.
func LeftJoin(e *State, keysCSV, columnsCSV string, rowData ...*ValueSet) (*ValueSet, error) {
	res := &ValueSet{}
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
			for _, val := range r.Elements {
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
			for _, val := range r.Elements {
				if group.Subset(val.Group) {
					t.Rows[rowIndex][keyWidth+i] = val.Value.Value()
				}
			}
		}
	}
	return &ValueSet{
		Elements: []*Element{
			{Value: t},
		},
	}, nil
}

// Shift is the "shift" function in the expression language.
func Shift(e *State, seriesSet *ValueSet, d string) (*ValueSet, error) {
	dur, err := opentsdb.ParseDuration(d)
	if err != nil {
		return seriesSet, err
	}
	for _, result := range seriesSet.Elements {
		newSeries := make(Series)
		for t, v := range result.Value.Value().(Series) {
			newSeries[t.Add(time.Duration(dur))] = v
		}
		result.Group["shift"] = d
		result.Value = newSeries
	}
	return seriesSet, nil
}

// Duration is the "d" function in the expression language.
func Duration(e *State, d string) (*ValueSet, error) {
	duration, err := opentsdb.ParseDuration(d)
	if err != nil {
		return nil, err
	}
	return &ValueSet{
		Elements: []*Element{
			{Value: Scalar(duration.Seconds())},
		},
	}, nil
}

// ToDuration is the "tod" function in the expression language.
func ToDuration(e *State, sec float64) (*ValueSet, error) {
	d := opentsdb.Duration(time.Duration(int64(sec)) * time.Second)
	return &ValueSet{
		Elements: []*Element{
			{Value: String(d.HumanString())},
		},
	}, nil
}

func dropValues(e *State, seriesSet *ValueSet, thresholdSet *ValueSet, dropFunction func(float64, float64) bool) (*ValueSet, error) {
	dropSeries := func(res *ValueSet, s *Element, floats []float64) error {
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
		res.Append(s)
		return nil
	}
	return match(dropSeries, seriesSet, thresholdSet)
}

// DropGe is the "dropge" function in the expression language.
func DropGe(e *State, seriesSet *ValueSet, thresholdSet *ValueSet) (*ValueSet, error) {
	dropFunction := func(value float64, threshold float64) bool { return value >= threshold }
	return dropValues(e, seriesSet, thresholdSet, dropFunction)
}

// DropG is the "dropg" function in the expression language.
func DropG(e *State, seriesSet *ValueSet, thresholdSet *ValueSet) (*ValueSet, error) {
	dropFunction := func(value float64, threshold float64) bool { return value > threshold }
	return dropValues(e, seriesSet, thresholdSet, dropFunction)
}

// DropLe is the "drople" function in the expression language.
func DropLe(e *State, seriesSet *ValueSet, thresholdSet *ValueSet) (*ValueSet, error) {
	dropFunction := func(value float64, threshold float64) bool { return value <= threshold }
	return dropValues(e, seriesSet, thresholdSet, dropFunction)
}

// DropL is the "dropl" function in the expression language.
func DropL(e *State, seriesSet *ValueSet, thresholdSet *ValueSet) (*ValueSet, error) {
	dropFunction := func(value float64, threshold float64) bool { return value < threshold }
	return dropValues(e, seriesSet, thresholdSet, dropFunction)
}

// DropNA is the "dropna" function in the expression language.
func DropNA(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	dropFunction := func(value float64, threshold float64) bool {
		return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
	}
	return dropValues(e, seriesSet, FromScalar(0), dropFunction)
}

// FromScalar takes a single float64 and turns it into an numberSet
// with no tags.
func FromScalar(f float64) *ValueSet {
	return &ValueSet{
		Elements: ElementSlice{
			&Element{
				Value: Number(f),
			},
		},
	}
}

// match allows functions to union operations internally
func match(f func(res *ValueSet, series *Element, floats []float64) error, seriesSet *ValueSet, numberSets ...*ValueSet) (*ValueSet, error) {
	res := *seriesSet
	res.Elements = nil
	for _, s := range seriesSet.Elements {
		var floats []float64
		for _, numberSet := range numberSets {
			for _, n := range numberSet.Elements {
				if len(n.Group) == 0 || s.Group.Overlaps(n.Group) {
					floats = append(floats, float64(n.Value.(Number)))
					break
				}
			}
		}
		if len(floats) != len(numberSets) {
			if !seriesSet.IgnoreUnjoined {
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

// ReduceSeriesSet applies the reducerFunc to each series in the set. It then calls match so function
// can use internal unions to handle the case where it is passed a numberSet as an additional
// argument to the reducer function.
func ReduceSeriesSet(e *State, seriesSet *ValueSet, reducerFunc func(Series, ...float64) float64, args ...*ValueSet) (*ValueSet, error) {
	f := func(res *ValueSet, s *Element, floats []float64) error {
		switch tp := s.Value.(type) {
		case Series:
			t := s.Value.(Series)
			if len(t) == 0 {
				return nil
			}
			s.Value = Number(reducerFunc(t, floats...))
			res.Append(s)
			return nil
		default:
			return fmt.Errorf(`Unsupported type passed to ReduceSeriesSet for alarm [%s].`+
				`Want: Series, got: %s. It can happen when we can't unjoin values.`+
				`Please set IgnoreUnjoined and/or IgnoreOtherUnjoined for distiguish this error.`,
				e.Origin, reflect.TypeOf(tp).String(),
			)
		}

	}
	return match(f, seriesSet, args...)
}

// Abs is the "abs" function in the expression language. It can take a numberSet or a seriesSet.
func Abs(e *State, set *ValueSet) *ValueSet {
	for _, s := range set.Elements {
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

// Diff is the "diff" function in the expression language.
func Diff(e *State, seriesSet *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, diffSeries)
}

func diffSeries(dps Series, args ...float64) float64 {
	return seriesLastValue(dps) - seriesFirstValue(dps)
}

// Avg is the "avg" function in the expression language.
func Avg(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, SeriesAvg)
}

// SeriesAvg returns the mean of x. It exported to make it available
// to tsdb packages.
func SeriesAvg(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	a /= float64(len(dps))
	return
}

// CCount is the "cCount" function in the expression language.
func CCount(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, cCountSeries)
}

func cCountSeries(dps Series, args ...float64) (a float64) {
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

// TimeDelta is the "timedelta" function in the expression language.
func TimeDelta(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	for _, res := range seriesSet.Elements {
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
	return seriesSet, nil
}

// Sum is the "sum" function in the expression language.
func Sum(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, sumSeries)
}

func sumSeries(dps Series, args ...float64) (a float64) {
	for _, v := range dps {
		a += float64(v)
	}
	return
}

// Des is the "des" function the expression language.
func Des(e *State, seriesSet *ValueSet, alpha float64, beta float64) *ValueSet {
	for _, res := range seriesSet.Elements {
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
	return seriesSet
}

// Streak is the "streak" function the expression language.
func Streak(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, streakSeries)
}

func streakSeries(dps Series, args ...float64) (a float64) {
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

// Dev is the "dev" function in the expression language.
func Dev(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, devSeries)
}

// devSeries returns the sample standard deviation of x.
func devSeries(dps Series, args ...float64) (d float64) {
	if len(dps) == 1 {
		return 0
	}
	a := SeriesAvg(dps)
	for _, v := range dps {
		d += math.Pow(float64(v)-a, 2)
	}
	d /= float64(len(dps) - 1)
	return math.Sqrt(d)
}

// Length in the "len" function in the expression language.
func Length(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, seriesLength)
}

func seriesLength(dps Series, args ...float64) (a float64) {
	return float64(len(dps))
}

// Last is the "last" function in the expression language.
func Last(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, seriesLastValue)
}

func seriesLastValue(dps Series, args ...float64) (a float64) {
	var last time.Time
	for k, v := range dps {
		if k.After(last) {
			a = v
			last = k
		}
	}
	return
}

// First in the "first" function in the expression language.
func First(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, seriesFirstValue)
}

func seriesFirstValue(dps Series, args ...float64) (a float64) {
	var first time.Time
	for k, v := range dps {
		if k.Before(first) || first.IsZero() {
			a = v
			first = k
		}
	}
	return
}

// Since is the "since" function in the expression language.
func Since(e *State, seriesSet *ValueSet) (*ValueSet, error) {
	return ReduceSeriesSet(e, seriesSet, e.seriesSince)
}

func (e *State) seriesSince(dps Series, args ...float64) (a float64) {
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

// ForecastLR is the "forecast_lr" function in the expression language.
func ForecastLR(e *State, seriesSet *ValueSet, y *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, e.forecastLRSeries, y)
}

// forecastLRSeries returns the number of seconds a linear regression predicts the
// series will take to reach y_val.
func (e *State) forecastLRSeries(dps Series, args ...float64) float64 {
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

// LineLR is the "linelr" function in the expression language.
func LineLR(e *State, seriesSet *ValueSet, d string) (*ValueSet, error) {
	dur, err := opentsdb.ParseDuration(d)
	if err != nil {
		return seriesSet, err
	}
	for _, res := range seriesSet.Elements {
		res.Value = lineLRSeries(res.Value.(Series), time.Duration(dur))
		res.Group.Merge(opentsdb.TagSet{"regression": "line"})
	}
	return seriesSet, nil
}

// lineLRSeries generates a series representing the line up to duration in the future.
func lineLRSeries(dps Series, d time.Duration) Series {
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

// Percentile is the "percentile" function in the expression language.
func Percentile(e *State, seriesSet *ValueSet, p *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, seriesPercentile, p)
}

// Min is the "min" function in the expression language.
func Min(e *State, seriesSet *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, seriesPercentile, FromScalar(0))
}

// Median is the "median" function in the expression language.
func Median(e *State, seriesSet *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, seriesPercentile, FromScalar(.5))
}

// Max is the "max" function in the expression language.
func Max(e *State, seriesSet *ValueSet) (r *ValueSet, err error) {
	return ReduceSeriesSet(e, seriesSet, seriesPercentile, FromScalar(1))
}

// seriesPercentile returns the value at the corresponding seriesPercentile between 0 and 1.
// Min and Max can be simulated using p <= 0 and p >= 1, respectively.
func seriesPercentile(dps Series, args ...float64) (a float64) {
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

// Rename is the "rename" function in the expression language.
func Rename(e *State, set *ValueSet, s string) (*ValueSet, error) {
	for _, section := range strings.Split(s, ",") {
		kv := strings.Split(section, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("error passing groups")
		}
		oldKey, newKey := kv[0], kv[1]
		for _, res := range set.Elements {
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

// AddTags is the "addtags" function in the expression language.
func AddTags(e *State, set *ValueSet, s string) (*ValueSet, error) {
	if s == "" {
		return set, nil
	}
	tagSetToAdd, err := opentsdb.ParseTags(s)
	if err != nil {
		return nil, err
	}
	for tagKey, tagValue := range tagSetToAdd {
		for _, res := range set.Elements {
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

// Ungroup is the "ungroup" function in the expression language.
func Ungroup(e *State, d *ValueSet) (*ValueSet, error) {
	if len(d.Elements) != 1 {
		return nil, fmt.Errorf("ungroup: requires exactly one group")
	}
	return &ValueSet{
		Elements: ElementSlice{
			&Element{
				Value: Scalar(d.Elements[0].Value.Value().(Number)),
			},
		},
	}, nil
}

// Transpose is the "t" function in the expression language.
func Transpose(e *State, numberSet *ValueSet, gp string) (*ValueSet, error) {
	gps := strings.Split(gp, ",")
	m := make(map[string]*Element)
	for _, v := range numberSet.Elements {
		ts := make(opentsdb.TagSet)
		for k, v := range v.Group {
			for _, b := range gps {
				if k == b {
					ts[k] = v
				}
			}
		}
		if _, ok := m[ts.String()]; !ok {
			m[ts.String()] = &Element{
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
	var r ValueSet
	for _, res := range m {
		r.Append(res)
	}
	return &r, nil
}

// ParseDurationPair is a helper to parse Bosun/OpenTSDB style duration strings that are often
// the last two arguments of tsdb query functions. It uses the State object's now property
// and returns absolute start and end times
func ParseDurationPair(e *State, startDuration, endDuration string) (start, end time.Time, err error) {
	sd, err := opentsdb.ParseDuration(startDuration)
	if err != nil {
		return
	}
	var ed opentsdb.Duration
	if endDuration != "" {
		ed, err = opentsdb.ParseDuration(endDuration)
		if err != nil {
			return
		}
	}
	return e.now.Add(time.Duration(-sd)), e.now.Add(time.Duration(-ed)), nil
}
