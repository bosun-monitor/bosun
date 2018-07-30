package expr // import "bosun.org/cmd/bosun/expr"

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"bosun.org/annotate/backend"
	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/cmd/bosun/search"
	"bosun.org/collect"
	"bosun.org/graphite"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/influxdata/influxdb/client/v2"
)

type State struct {
	*Expr
	now                time.Time
	enableComputations bool
	unjoinedOk         bool
	autods             int
	vValue             float64

	*Backends

	// Bosun Internal
	*BosunProviders

	// Graphite
	graphiteQueries []graphite.Request

	// OpenTSDB
	tsdbQueries []opentsdb.Request
}

type Backends struct {
	TSDBContext     opentsdb.Context
	GraphiteContext graphite.Context
	ElasticHosts    ElasticHosts
	InfluxConfig    client.HTTPConfig
	ElasticConfig   ElasticConfig
	AzureMonitor    AzureMonitorClients
}

type BosunProviders struct {
	Squelched func(tags opentsdb.TagSet) bool
	Search    *search.Search
	History   AlertStatusProvider
	Cache     *cache.Cache
	Annotate  backend.Backend
}

// Alert Status Provider is used to provide information about alert results.
// This facilitates alerts referencing other alerts, even when they go unknown or unevaluated.
type AlertStatusProvider interface {
	GetUnknownAndUnevaluatedAlertKeys(alertName string) (unknown, unevaluated []models.AlertKey)
}

var ErrUnknownOp = fmt.Errorf("expr: unknown op type")

type Expr struct {
	*parse.Tree
}

func (e *Expr) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// New creates a new expression tree
func New(expr string, funcs ...map[string]parse.Func) (*Expr, error) {
	funcs = append(funcs, builtins)
	t, err := parse.Parse(expr, funcs...)
	if err != nil {
		return nil, err
	}
	e := &Expr{
		Tree: t,
	}
	return e, nil
}

// Execute applies a parse expression to the specified OpenTSDB context, and
// returns one result per group. T may be nil to ignore timings.
func (e *Expr) Execute(backends *Backends, providers *BosunProviders, T miniprofiler.Timer, now time.Time, autods int, unjoinedOk bool) (r *Results, queries []opentsdb.Request, err error) {
	if providers.Squelched == nil {
		providers.Squelched = func(tags opentsdb.TagSet) bool {
			return false
		}
	}
	s := &State{
		Expr:           e,
		now:            now,
		autods:         autods,
		unjoinedOk:     unjoinedOk,
		Backends:       backends,
		BosunProviders: providers,
	}
	return e.ExecuteState(s, T)
}

func (e *Expr) ExecuteState(s *State, T miniprofiler.Timer) (r *Results, queries []opentsdb.Request, err error) {
	defer errRecover(&err)
	if T == nil {
		T = new(miniprofiler.Profile)
	} else {
		s.enableComputations = true
	}
	T.Step("expr execute", func(T miniprofiler.Timer) {
		r = s.walk(e.Tree.Root, T)
	})
	queries = s.tsdbQueries
	return
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			slog.Infof("%s: %s", e, debug.Stack())
			panic(e)
		case error:
			*errp = err
		default:
			slog.Infof("%s: %s", e, debug.Stack())
			panic(e)
		}
	}
}

func marshalFloat(n float64) ([]byte, error) {
	if math.IsNaN(n) {
		return json.Marshal("NaN")
	} else if math.IsInf(n, 1) {
		return json.Marshal("+Inf")
	} else if math.IsInf(n, -1) {
		return json.Marshal("-Inf")
	}
	return json.Marshal(n)
}

type Value interface {
	Type() models.FuncType
	Value() interface{}
}

type Number float64

func (n Number) Type() models.FuncType        { return models.TypeNumberSet }
func (n Number) Value() interface{}           { return n }
func (n Number) MarshalJSON() ([]byte, error) { return marshalFloat(float64(n)) }

type Scalar float64

func (s Scalar) Type() models.FuncType        { return models.TypeScalar }
func (s Scalar) Value() interface{}           { return s }
func (s Scalar) MarshalJSON() ([]byte, error) { return marshalFloat(float64(s)) }

type String string

func (s String) Type() models.FuncType { return models.TypeString }
func (s String) Value() interface{}    { return s }

type NumberExpr Expr

func (s NumberExpr) Type() models.FuncType { return models.TypeNumberExpr }
func (s NumberExpr) Value() interface{}    { return s }

//func (s String) MarshalJSON() ([]byte, error) { return json.Marshal(s) }

// Series is the standard form within bosun to represent timeseries data.
type Series map[time.Time]float64

func (s Series) Type() models.FuncType { return models.TypeSeriesSet }
func (s Series) Value() interface{}    { return s }

func (s Series) MarshalJSON() ([]byte, error) {
	r := make(map[string]interface{}, len(s))
	for k, v := range s {
		r[fmt.Sprint(k.Unix())] = Scalar(v)
	}
	return json.Marshal(r)
}

func (a Series) Equal(b Series) bool {
	return reflect.DeepEqual(a, b)
}

// See the elastic#.go files for ESQuery

func (e ESQuery) Type() models.FuncType { return models.TypeESQuery }
func (e ESQuery) Value() interface{}    { return e }
func (e ESQuery) MarshalJSON() ([]byte, error) {
	source, err := e.Query.Source()
	if err != nil {
		return nil, err
	}
	return json.Marshal(source)
}

type ESIndexer struct {
	TimeField string
	Generate  func(startDuration, endDuration *time.Time) []string
}

func (e ESIndexer) Type() models.FuncType { return models.TypeESIndexer }
func (e ESIndexer) Value() interface{}    { return e }
func (e ESIndexer) MarshalJSON() ([]byte, error) {
	return json.Marshal("ESGenerator")
}

type Table struct {
	Columns []string
	Rows    [][]interface{}
}

func (t Table) Type() models.FuncType { return models.TypeTable }
func (t Table) Value() interface{}    { return t }

func (a AzureResources) Type() models.FuncType { return models.TypeAzureResourceList }
func (a AzureResources) Value() interface{}    { return a }

type SortablePoint struct {
	T time.Time
	V float64
}

// SortableSeries is an alternative datastructure for timeseries data,
// which stores points in a time-ordered fashion instead of a map.
// see discussion at https://github.com/bosun-monitor/bosun/pull/699
type SortableSeries []SortablePoint

func (s SortableSeries) Len() int           { return len(s) }
func (s SortableSeries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SortableSeries) Less(i, j int) bool { return s[i].T.Before(s[j].T) }

func NewSortedSeries(dps Series) SortableSeries {
	series := make(SortableSeries, 0, len(dps))
	for t, v := range dps {
		series = append(series, SortablePoint{t, v})
	}
	sort.Sort(series)
	return series
}

type Result struct {
	models.Computations
	Value
	Group opentsdb.TagSet
}

type Results struct {
	Results ResultSlice
	// If true, ungrouped joins from this set will be ignored.
	IgnoreUnjoined bool
	// If true, ungrouped joins from the other set will be ignored.
	IgnoreOtherUnjoined bool
	// If non nil, will set any NaN value to it.
	NaNValue *float64
}

// Equal inspects if two results have the same content
// error will return why they are not equal if they
// are not equal
func (a *Results) Equal(b *Results) (bool, error) {
	if len(a.Results) != len(b.Results) {
		return false, fmt.Errorf("unequal number of results: length a: %v, length b: %v", len(a.Results), len(b.Results))
	}
	if a.IgnoreUnjoined != b.IgnoreUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", a.IgnoreUnjoined, b.IgnoreUnjoined)
	}
	if a.IgnoreOtherUnjoined != b.IgnoreOtherUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", a.IgnoreOtherUnjoined, b.IgnoreOtherUnjoined)
	}
	if a.NaNValue != a.NaNValue {
		return false, fmt.Errorf("NaNValue does not match a: %v, b: %v", a.NaNValue, b.NaNValue)
	}
	sortedA := ResultSliceByGroup(a.Results)
	sort.Sort(sortedA)
	sortedB := ResultSliceByGroup(b.Results)
	sort.Sort(sortedB)
	for i, result := range sortedA {
		for ic, computation := range result.Computations {
			if computation != sortedB[i].Computations[ic] {
				return false, fmt.Errorf("mismatched computation a: %v, b: %v", computation, sortedB[ic])
			}
		}
		if !result.Group.Equal(sortedB[i].Group) {
			return false, fmt.Errorf("mismatched groups a: %v, b: %v", result.Group, sortedB[i].Group)
		}
		switch t := result.Value.(type) {
		case Number, Scalar, String:
			if result.Value != sortedB[i].Value {
				return false, fmt.Errorf("values do not match a: %v, b: %v", result.Value, sortedB[i].Value)
			}
		case Series:
			if !t.Equal(sortedB[i].Value.(Series)) {
				return false, fmt.Errorf("mismatched series in result (Group: %s) a: %v, b: %v", result.Group, t, sortedB[i].Value.(Series))
			}
		default:
			panic(fmt.Sprintf("can't compare results with type %T", t))
		}

	}
	return true, nil
}

type ResultSlice []*Result

type ResultSliceByGroup ResultSlice

type ResultSliceByValue ResultSlice

func (r *Results) NaN() Number {
	if r.NaNValue != nil {
		return Number(*r.NaNValue)
	}
	return Number(math.NaN())
}

func (r ResultSlice) DescByValue() ResultSlice {
	for _, v := range r {
		if _, ok := v.Value.(Number); !ok {
			return r
		}
	}
	c := r[:]
	sort.Sort(sort.Reverse(ResultSliceByValue(c)))
	return c
}

// Filter returns a slice with only the results that have a tagset that conforms to the given key/value pair restrictions
func (r ResultSlice) Filter(filter opentsdb.TagSet) ResultSlice {
	output := make(ResultSlice, 0, len(r))
	for _, res := range r {
		if res.Group.Compatible(filter) {
			output = append(output, res)
		}
	}
	return output
}

func (r ResultSliceByValue) Len() int           { return len(r) }
func (r ResultSliceByValue) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ResultSliceByValue) Less(i, j int) bool { return r[i].Value.(Number) < r[j].Value.(Number) }

func (r ResultSliceByGroup) Len() int           { return len(r) }
func (r ResultSliceByGroup) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ResultSliceByGroup) Less(i, j int) bool { return r[i].Group.String() < r[j].Group.String() }

func (e *State) AddComputation(r *Result, text string, value interface{}) {
	if !e.enableComputations {
		return
	}
	r.Computations = append(r.Computations, models.Computation{Text: opentsdb.ReplaceTags(text, r.Group), Value: value})
}

type Union struct {
	models.Computations
	A, B  Value
	Group opentsdb.TagSet
}

// wrap creates a new Result with a nil group and given value.
func wrap(v float64) *Results {
	return &Results{
		Results: []*Result{
			{
				Value: Scalar(v),
				Group: nil,
			},
		},
	}
}

func (u *Union) ExtendComputations(o *Result) {
	u.Computations = append(u.Computations, o.Computations...)
}

// union returns the combination of a and b where one is a subset of the other.
func (e *State) union(a, b *Results, expression string) []*Union {
	const unjoinedGroup = "unjoined group (%v)"
	var us []*Union
	if len(a.Results) == 0 || len(b.Results) == 0 {
		return us
	}
	am := make(map[*Result]bool, len(a.Results))
	bm := make(map[*Result]bool, len(b.Results))
	for _, ra := range a.Results {
		am[ra] = true
	}
	for _, rb := range b.Results {
		bm[rb] = true
	}
	var group opentsdb.TagSet
	for _, ra := range a.Results {
		for _, rb := range b.Results {
			if ra.Group.Equal(rb.Group) || len(ra.Group) == 0 || len(rb.Group) == 0 {
				g := ra.Group
				if len(ra.Group) == 0 {
					g = rb.Group
				}
				group = g
			} else if len(ra.Group) == len(rb.Group) {
				continue
			} else if ra.Group.Subset(rb.Group) {
				group = ra.Group
			} else if rb.Group.Subset(ra.Group) {
				group = rb.Group
			} else {
				continue
			}
			delete(am, ra)
			delete(bm, rb)
			u := &Union{
				A:     ra.Value,
				B:     rb.Value,
				Group: group,
			}
			u.ExtendComputations(ra)
			u.ExtendComputations(rb)
			us = append(us, u)
		}
	}
	if !e.unjoinedOk {
		if !a.IgnoreUnjoined && !b.IgnoreOtherUnjoined {
			for r := range am {
				u := &Union{
					A:     r.Value,
					B:     b.NaN(),
					Group: r.Group,
				}
				e.AddComputation(r, expression, fmt.Sprintf(unjoinedGroup, u.B))
				u.ExtendComputations(r)
				us = append(us, u)
			}
		}
		if !b.IgnoreUnjoined && !a.IgnoreOtherUnjoined {
			for r := range bm {
				u := &Union{
					A:     a.NaN(),
					B:     r.Value,
					Group: r.Group,
				}
				e.AddComputation(r, expression, fmt.Sprintf(unjoinedGroup, u.A))
				u.ExtendComputations(r)
				us = append(us, u)
			}
		}
	}
	return us
}

func (e *State) walk(node parse.Node, T miniprofiler.Timer) *Results {
	var res *Results
	switch node := node.(type) {
	case *parse.NumberNode:
		res = wrap(node.Float64)
	case *parse.BinaryNode:
		res = e.walkBinary(node, T)
	case *parse.UnaryNode:
		res = e.walkUnary(node, T)
	case *parse.FuncNode:
		res = e.walkFunc(node, T)
	case *parse.ExprNode:
		res = e.walkExpr(node, T)
	case *parse.PrefixNode:
		res = e.walkPrefix(node, T)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
	return res
}

func (e *State) walkExpr(node *parse.ExprNode, T miniprofiler.Timer) *Results {
	return &Results{
		Results: ResultSlice{
			&Result{
				Value: NumberExpr{node.Tree},
			},
		},
	}
}

func (e *State) walkBinary(node *parse.BinaryNode, T miniprofiler.Timer) *Results {
	ar := e.walk(node.Args[0], T)
	br := e.walk(node.Args[1], T)
	res := Results{
		IgnoreUnjoined:      ar.IgnoreUnjoined || br.IgnoreUnjoined,
		IgnoreOtherUnjoined: ar.IgnoreOtherUnjoined || br.IgnoreOtherUnjoined,
	}
	T.Step("walkBinary: "+node.OpStr, func(T miniprofiler.Timer) {
		u := e.union(ar, br, node.String())
		for _, v := range u {
			var value Value
			r := &Result{
				Group:        v.Group,
				Computations: v.Computations,
			}
			switch at := v.A.(type) {
			case Scalar:
				switch bt := v.B.(type) {
				case Scalar:
					n := Scalar(operate(node.OpStr, float64(at), float64(bt)))
					e.AddComputation(r, node.String(), Number(n))
					value = n
				case Number:
					n := Number(operate(node.OpStr, float64(at), float64(bt)))
					e.AddComputation(r, node.String(), n)
					value = n
				case Series:
					s := make(Series)
					for k, v := range bt {
						s[k] = operate(node.OpStr, float64(at), float64(v))
					}
					value = s
				default:
					panic(ErrUnknownOp)
				}
			case Number:
				switch bt := v.B.(type) {
				case Scalar:
					n := Number(operate(node.OpStr, float64(at), float64(bt)))
					e.AddComputation(r, node.String(), Number(n))
					value = n
				case Number:
					n := Number(operate(node.OpStr, float64(at), float64(bt)))
					e.AddComputation(r, node.String(), n)
					value = n
				case Series:
					s := make(Series)
					for k, v := range bt {
						s[k] = operate(node.OpStr, float64(at), float64(v))
					}
					value = s
				default:
					panic(ErrUnknownOp)
				}
			case Series:
				switch bt := v.B.(type) {
				case Number, Scalar:
					bv := reflect.ValueOf(bt).Float()
					s := make(Series)
					for k, v := range at {
						s[k] = operate(node.OpStr, float64(v), bv)
					}
					value = s
				case Series:
					s := make(Series)
					for k, av := range at {
						if bv, ok := bt[k]; ok {
							s[k] = operate(node.OpStr, av, bv)
						}
					}
					value = s
				default:
					panic(ErrUnknownOp)
				}
			default:
				panic(ErrUnknownOp)
			}
			r.Value = value
			res.Results = append(res.Results, r)
		}
	})
	return &res
}

func operate(op string, a, b float64) (r float64) {
	// Test short circuit before NaN.
	switch op {
	case "||":
		if a != 0 {
			return 1
		}
	case "&&":
		if a == 0 {
			return 0
		}
	}
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	switch op {
	case "+":
		r = a + b
	case "*":
		r = a * b
	case "-":
		r = a - b
	case "/":
		r = a / b
	case "**":
		r = math.Pow(a, b)
	case "%":
		r = math.Mod(a, b)
	case "==":
		if a == b {
			r = 1
		} else {
			r = 0
		}
	case ">":
		if a > b {
			r = 1
		} else {
			r = 0
		}
	case "!=":
		if a != b {
			r = 1
		} else {
			r = 0
		}
	case "<":
		if a < b {
			r = 1
		} else {
			r = 0
		}
	case ">=":
		if a >= b {
			r = 1
		} else {
			r = 0
		}
	case "<=":
		if a <= b {
			r = 1
		} else {
			r = 0
		}
	case "||":
		if a != 0 || b != 0 {
			r = 1
		} else {
			r = 0
		}
	case "&&":
		if a != 0 && b != 0 {
			r = 1
		} else {
			r = 0
		}
	default:
		panic(fmt.Errorf("expr: unknown operator %s", op))
	}
	return
}

func (e *State) walkUnary(node *parse.UnaryNode, T miniprofiler.Timer) *Results {
	a := e.walk(node.Arg, T)
	T.Step("walkUnary: "+node.OpStr, func(T miniprofiler.Timer) {
		for _, r := range a.Results {
			if an, aok := r.Value.(Scalar); aok && math.IsNaN(float64(an)) {
				r.Value = Scalar(math.NaN())
				continue
			}
			switch rt := r.Value.(type) {
			case Scalar:
				r.Value = Scalar(uoperate(node.OpStr, float64(rt)))
			case Number:
				r.Value = Number(uoperate(node.OpStr, float64(rt)))
			case Series:
				s := make(Series)
				for k, v := range rt {
					s[k] = uoperate(node.OpStr, float64(v))
				}
				r.Value = s
			default:
				panic(ErrUnknownOp)
			}
		}
	})
	return a
}

func uoperate(op string, a float64) (r float64) {
	switch op {
	case "!":
		if a == 0 {
			r = 1
		} else {
			r = 0
		}
	case "-":
		r = -a
	default:
		panic(fmt.Errorf("expr: unknown operator %s", op))
	}
	return
}

func (e *State) walkPrefix(node *parse.PrefixNode, T miniprofiler.Timer) *Results {
	key := strings.TrimPrefix(node.Text, "[")
	key = strings.TrimSuffix(key, "]")
	key, _ = strconv.Unquote(key)
	switch node := node.Arg.(type) {
	case *parse.FuncNode:
		if node.F.PrefixEnabled {
			node.Prefix = key
			node.F.PrefixKey = true
		}
		return e.walk(node, T)
	default:
		panic(fmt.Errorf("expr: prefix can only be append to a FuncNode"))
	}
}

func (e *State) walkFunc(node *parse.FuncNode, T miniprofiler.Timer) *Results {
	var res *Results
	T.Step("func: "+node.Name, func(T miniprofiler.Timer) {
		var in []reflect.Value
		for i, a := range node.Args {
			var v interface{}
			switch t := a.(type) {
			case *parse.StringNode:
				v = t.Text
			case *parse.NumberNode:
				v = t.Float64
			case *parse.FuncNode:
				v = extract(e.walkFunc(t, T))
			case *parse.UnaryNode:
				v = extract(e.walkUnary(t, T))
			case *parse.BinaryNode:
				v = extract(e.walkBinary(t, T))
			case *parse.ExprNode:
				v = e.walkExpr(t, T)
			case *parse.PrefixNode:
				v = extract(e.walkPrefix(t, T))
			default:
				panic(fmt.Errorf("expr: unknown func arg type"))
			}

			var argType models.FuncType
			if i >= len(node.F.Args) {
				if !node.F.VArgs {
					panic("expr: shouldn't be here, more args then expected and not variable argument type func")
				}
				argType = node.F.Args[node.F.VArgsPos]
			} else {
				argType = node.F.Args[i]
			}
			if f, ok := v.(float64); ok && (argType == models.TypeNumberSet || argType == models.TypeVariantSet) {
				v = fromScalar(f)
			}
			in = append(in, reflect.ValueOf(v))
		}

		f := reflect.ValueOf(node.F.F)
		fr := []reflect.Value{}

		if node.F.PrefixEnabled {
			if !node.F.PrefixKey {
				fr = f.Call(append([]reflect.Value{reflect.ValueOf("default"), reflect.ValueOf(e), reflect.ValueOf(T)}, in...))
			} else {
				fr = f.Call(append([]reflect.Value{reflect.ValueOf(node.Prefix), reflect.ValueOf(e), reflect.ValueOf(T)}, in...))
			}
		} else {
			fr = f.Call(append([]reflect.Value{reflect.ValueOf(e), reflect.ValueOf(T)}, in...))
		}

		res = fr[0].Interface().(*Results)
		if len(fr) > 1 && !fr[1].IsNil() {
			err := fr[1].Interface().(error)
			if err != nil {
				panic(err)
			}
		}
		if node.Return() == models.TypeNumberSet {
			for _, r := range res.Results {
				e.AddComputation(r, node.String(), r.Value.(Number))
			}
		}
	})
	return res
}

// extract will return a float64 if res contains exactly one scalar or a ESQuery if that is the type
func extract(res *Results) interface{} {
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeScalar {
		return float64(res.Results[0].Value.Value().(Scalar))
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeESQuery {
		return res.Results[0].Value.Value()
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeAzureResourceList {
		return res.Results[0].Value.Value()
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeESIndexer {
		return res.Results[0].Value.Value()
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeString {
		return string(res.Results[0].Value.Value().(String))
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeNumberExpr {
		return res.Results[0].Value.Value()
	}
	return res
}

// collectCache is a helper function for collecting metrics on
// the expression cache
func collectCacheHit(c *cache.Cache, qType string, hit bool) {
	if c == nil {
		return // if no cache
	}
	tags := opentsdb.TagSet{"query_type": qType, "name": c.Name}
	if hit {
		collect.Add("expr_cache.hit_by_type", tags, 1)
		return
	}
	collect.Add("expr_cache.miss_by_type", tags, 1)
}

func init() {
	metadata.AddMetricMeta("bosun.expr_cache.hit_by_type", metadata.Counter, metadata.Request,
		"The number of hits to Bosun's expression query cache that resulted in a cache hit.")
	metadata.AddMetricMeta("bosun.expr_cache.miss_by_type", metadata.Counter, metadata.Request,
		"The number of hits to Bosun's expression query cache that resulted in a cache miss.")
}
