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
	"bosun.org/cmd/bosun/expr/tsdbs"
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

// State contains various flags, properties, and providers that change how
// the embedded Expression will execute.
type State struct {
	*Expr
	// now a specified time to represent current time for the expression.
	// Relative times in the expression language use now to calculate start and end times.
	now                time.Time
	enableComputations bool
	unjoinedOk         bool
	autods             int
	vValue             float64

	// Origin allows the source of the expression to be identified for logging and debugging
	Origin string

	Timer miniprofiler.Timer // a profiler for capturing the performance of various functions in the expression

	*TSDBs

	// Bosun Internal
	*BosunProviders

	// Graphite
	GraphiteQueries []graphite.Request

	// OpenTSDB
	OpenTSDBQueries []opentsdb.Request
}

// TSDBs contains the information needed by tsdb packages to be able to query their respective databases.
type TSDBs struct {
	Azure      tsdbs.AzureMonitorClients
	Elastic    tsdbs.ElasticHosts
	Graphite   graphite.Context
	Influx     client.HTTPConfig
	OpenTSDB   opentsdb.Context
	Prometheus tsdbs.PromClients
}

// BosunProviders is a collection of various Providers that are availble to
// for Expressions
type BosunProviders struct {
	// a function that can be used to identify tags that have squelched in Bosun's rule configuration
	Squelched func(tags opentsdb.TagSet) bool
	// a provider for Bosun's relation information about metrics and tags
	Search *search.Search
	// a provider for the alert() expression function
	History AlertStatusProvider
	// a provider for caching query results
	Cache *cache.Cache
	// a provider for Bosun annotations
	Annotate backend.Backend
}

// AlertStatusProvider is used to provide information about alert results.
// This facilitates alerts referencing other alerts, even when they go unknown or unevaluated.
type AlertStatusProvider interface {
	GetUnknownAndUnevaluatedAlertKeys(alertName string) (unknown, unevaluated []models.AlertKey)
}

// ErrUnknownOp is the error message for an unknown operation type
var ErrUnknownOp = fmt.Errorf("expr: unknown op type")

// Expr embeds an expr/parse.Tree so methods can be attached to it
type Expr struct {
	*parse.Tree
}

// MarshalJSON allows the string representation of the expression to be rendered into JSON
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
func (e *Expr) Execute(tsdbs *TSDBs, providers *BosunProviders, T miniprofiler.Timer, now time.Time, autods int, unjoinedOk bool, origin string) (r *ResultSet, queries []opentsdb.Request, err error) {
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
		Origin:         origin,
		TSDBs:          tsdbs,
		BosunProviders: providers,
		Timer:          T,
	}
	return e.ExecuteState(s)
}

// ExecuteState execute the expression with the give State information and returns the results
func (e *Expr) ExecuteState(s *State) (r *ResultSet, queries []opentsdb.Request, err error) {
	defer errRecover(&err, s)
	if s.Timer == nil {
		s.Timer = new(miniprofiler.Profile)
	} else {
		s.enableComputations = true
	}
	s.Timer.Step("expr execute", func(T miniprofiler.Timer) {
		r = s.walk(e.Tree.Root)
	})
	queries = s.OpenTSDBQueries
	return
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error, s *State) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			slog.Errorf("Error: %s. Origin: %v. Expression: %s, Stack: %s", e, s.Origin, s.Expr, debug.Stack())
			panic(e)
		case error:
			*errp = err
		default:
			slog.Errorf("Error: %s. Origin: %v. Expression: %s, Stack: %s", e, s.Origin, s.Expr, debug.Stack())
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

// Value is the interface that all valid types in the expression language must
// fullfill
type Value interface {
	Type() models.FuncType // used to identify the type of the Value
	Value() interface{}    // the actual value
}

// Number is the expression type that should be the value type for all numbers
// in a ResultSet that is a numberSet
type Number float64

// Type returns the type representation so it fullfills the Value interface.
func (n Number) Type() models.FuncType { return models.TypeNumberSet }

// Value returns the value of the number and exists so it fullfills the Value interface.
func (n Number) Value() interface{} { return n }

// MarshalJSON allows the value of the number to be reprented in JSON while also
// allowing for NaN and InF values to be represented.
func (n Number) MarshalJSON() ([]byte, error) { return marshalFloat(float64(n)) }

// Scalar is the expression type that represents a single untagged number.
type Scalar float64

// Type returns the type representation so it fullfills the Value interface.
func (s Scalar) Type() models.FuncType { return models.TypeScalar }

// Value returns the value of the Scalar and exists so it fullfills the Value interface.
func (s Scalar) Value() interface{} { return s }

// MarshalJSON allows the value of the Scalar to be reprented in JSON while also
// allowing for NaN and InF values to be represented.
func (s Scalar) MarshalJSON() ([]byte, error) { return marshalFloat(float64(s)) }

// String is the expression type that represents a string.
type String string

// Type returns the type representation so it fullfills the Value interface.
func (s String) Type() models.FuncType { return models.TypeString }

// Value returns the value of the string and exists so it fullfills the Value interface.
func (s String) Value() interface{} { return s }

// NumberExpr represents a sub number expression in the expression language which is used with map().
type NumberExpr Expr

// Type returns the type representation so it fullfills the Value interface.
func (s NumberExpr) Type() models.FuncType { return models.TypeNumberExpr }

// Value returns the value of the NumberExpr and exists so it fullfills the Value interface.
func (s NumberExpr) Value() interface{} { return s }

// Info is a generic object in the expression language which is only used to return
// interative information to the user.
type Info []interface{}

// Type returns the type representation so it fullfills the Value interface.
func (i Info) Type() models.FuncType { return models.TypeInfo }

// Value returns the value of the Info object and exists so it fullfills the Value interface.
func (i Info) Value() interface{} { return i }

// Series is the standard form within bosun to represent timeseries data.
type Series map[time.Time]float64

// Type returns the type representation of the series so it fullfills the Value interface.
func (s Series) Type() models.FuncType { return models.TypeSeriesSet }

// Value returns the value of the Series and exists so it fullfills the Value interface.
func (s Series) Value() interface{} { return s }

// MarshalJSON returns the Series object in JSON.
func (s Series) MarshalJSON() ([]byte, error) {
	r := make(map[string]interface{}, len(s))
	for k, v := range s {
		r[fmt.Sprint(k.Unix())] = Scalar(v)
	}
	return json.Marshal(r)
}

// Equal returns if series s is equal to series b.
func (s Series) Equal(b Series) bool {
	return reflect.DeepEqual(s, b)
}

// Table is a return type that lines up with Grafana Tables. It can be viewed in the expression
// editor but is primarily meant for integration with Grafana. This type is not used for Alerting.
type Table struct {
	Columns []string
	Rows    [][]interface{}
}

// Type returns the type representation of the Table so it fullfills the Value interface.
func (t Table) Type() models.FuncType { return models.TypeTable }

// Value returns the value of the Series and exists so it fullfills the Value interface.
func (t Table) Value() interface{} { return t }

// SortableSeries is an alternative datastructure for timeseries data,
// which stores points in a time-ordered fashion instead of a map.
// see discussion at https://github.com/bosun-monitor/bosun/pull/699
type SortableSeries []SortablePoint

// SortablePoint in a member for Sortable Series.
type SortablePoint struct {
	T time.Time
	V float64
}

func (s SortableSeries) Len() int           { return len(s) }
func (s SortableSeries) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SortableSeries) Less(i, j int) bool { return s[i].T.Before(s[j].T) }

// NewSortedSeries takes a Series and returns it as a Sorted SortableSeries
func NewSortedSeries(dps Series) SortableSeries {
	series := make(SortableSeries, 0, len(dps))
	for t, v := range dps {
		series = append(series, SortablePoint{t, v})
	}
	sort.Sort(series)
	return series
}

// Result contains a single result and is generally contained within a Results Object.
type Result struct {
	// a list of sub computations for the expression. Collecting computations is not always enabled.
	models.Computations
	// The embedded Value which has a Value() method to get the actual Value, and Type() method to get the type
	Value
	// the tags for the result
	Group opentsdb.TagSet
}

// ResultSet contains the results of an expression operation or a expression function.
// It will also be the type returned from any completed evaluation of a complete expression.
// In addition it contains properties about how those results should behave in with certain Union
// operations.
//
// Each Result in the Results property should be of the same type. It is up to functions in the expression
// language to ensure the Results are a set with no conflicting entries and that all entries are of the same type.
type ResultSet struct {
	Results ResultSlice
	// If true, ungrouped joins from this set will be ignored.
	IgnoreUnjoined bool
	// If true, ungrouped joins from the other set will be ignored.
	IgnoreOtherUnjoined bool
	// If non nil, will set any NaN value to it when the nv() function is used.
	NaNValue *float64
}

// NaN returns the specified substitue value for NaN on the if one is present as a property on
// the ResultSet, else "NaN" is returned.
// The NaNValue property of the ResultSet is set when the nv() function is used in the expression language.
func (r *ResultSet) NaN() Number {
	if r.NaNValue != nil {
		return Number(*r.NaNValue)
	}
	return Number(math.NaN())
}

// Equal inspects if two ResultSets have the same content.
// An error will return explaing why they are not equal if they are not equal.
func (r *ResultSet) Equal(b *ResultSet) (bool, error) {
	if len(r.Results) != len(b.Results) {
		return false, fmt.Errorf("unequal number of results: length a: %v, length b: %v", len(r.Results), len(b.Results))
	}
	if r.IgnoreUnjoined != b.IgnoreUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", r.IgnoreUnjoined, b.IgnoreUnjoined)
	}
	if r.IgnoreOtherUnjoined != b.IgnoreOtherUnjoined {
		return false, fmt.Errorf("ignoreUnjoined flag does not match a: %v, b: %v", r.IgnoreOtherUnjoined, b.IgnoreOtherUnjoined)
	}
	if r.NaNValue != b.NaNValue {
		return false, fmt.Errorf("NaNValue does not match a: %v, b: %v", r.NaNValue, b.NaNValue)
	}
	sortedA := ResultSliceByGroup(r.Results)
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

// ResultSlice is a slice of Result Pointers.
type ResultSlice []*Result

// ResultSliceByGroup allows a ResultSlice to be sorted by Group (a.k.a. Tags).
type ResultSliceByGroup ResultSlice

// ResultSliceByValue allows a ResultSlice to be sorted by value.
type ResultSliceByValue ResultSlice

// DescByValue sorts a ResultSlice in Descending order by value.
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

// AddComputation adds a computation to the State if the state has the enabledComputations
// property enabled.
// If the computation has OpenTSDB style tags each individual computation will correctly represent the specific
// tag value for the computation. Otherwise it will be the raw text part of the expression that represents the
// computation as inputted.
func (e *State) AddComputation(r *Result, text string, value interface{}) {
	if !e.enableComputations {
		return
	}
	r.Computations = append(r.Computations, models.Computation{Text: opentsdb.ReplaceTags(text, r.Group), Value: value})
}

// AutoDS returns the auto-downsampling value for the expression. Not all TSDBs support the
// auto-downsampling feature.
func (e *State) AutoDS() int {
	return e.autods
}

// Now returns State's absolute time that represents the current time from the perspective
// of the expression State.
func (e *State) Now() time.Time {
	return e.now
}

// SetNow sets the Expression state's representation of now. This should not be
// used within expression functions.
func (e *State) SetNow(t time.Time) {
	e.now = t
	return
}

type Union struct {
	models.Computations
	A, B  Value
	Group opentsdb.TagSet
}

// wrap creates a new Result with a nil group and given value.
func wrap(v float64) *ResultSet {
	return &ResultSet{
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
func (e *State) union(a, b *ResultSet, expression string) []*Union {
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

func (e *State) walk(node parse.Node) *ResultSet {
	var res *ResultSet
	switch node := node.(type) {
	case *parse.NumberNode:
		res = wrap(node.Float64)
	case *parse.BinaryNode:
		res = e.walkBinary(node)
	case *parse.UnaryNode:
		res = e.walkUnary(node)
	case *parse.FuncNode:
		res = e.walkFunc(node)
	case *parse.ExprNode:
		res = e.walkExpr(node)
	case *parse.PrefixNode:
		res = e.walkPrefix(node)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
	return res
}

func (e *State) walkExpr(node *parse.ExprNode) *ResultSet {
	return &ResultSet{
		Results: ResultSlice{
			&Result{
				Value: NumberExpr{node.Tree},
			},
		},
	}
}

func (e *State) walkBinary(node *parse.BinaryNode) *ResultSet {
	ar := e.walk(node.Args[0])
	br := e.walk(node.Args[1])
	res := ResultSet{
		IgnoreUnjoined:      ar.IgnoreUnjoined || br.IgnoreUnjoined,
		IgnoreOtherUnjoined: ar.IgnoreOtherUnjoined || br.IgnoreOtherUnjoined,
	}
	e.Timer.Step("walkBinary: "+node.OpStr, func(T miniprofiler.Timer) {
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

func (e *State) walkUnary(node *parse.UnaryNode) *ResultSet {
	a := e.walk(node.Arg)
	e.Timer.Step("walkUnary: "+node.OpStr, func(T miniprofiler.Timer) {
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

func (e *State) walkPrefix(node *parse.PrefixNode) *ResultSet {
	key := strings.TrimPrefix(node.Text, "[")
	key = strings.TrimSuffix(key, "]")
	key, _ = strconv.Unquote(key)
	switch node := node.Arg.(type) {
	case *parse.FuncNode:
		if node.F.PrefixEnabled {
			node.Prefix = key
			node.F.PrefixKey = true
		}
		return e.walk(node)
	default:
		panic(fmt.Errorf("expr: prefix can only be append to a FuncNode"))
	}
}

func (e *State) walkFunc(node *parse.FuncNode) *ResultSet {
	var res *ResultSet
	e.Timer.Step("func: "+node.Name, func(T miniprofiler.Timer) {
		var in []reflect.Value
		for i, a := range node.Args {
			var v interface{}
			switch t := a.(type) {
			case *parse.StringNode:
				v = t.Text
			case *parse.NumberNode:
				v = t.Float64
			case *parse.FuncNode:
				v = extract(e.walkFunc(t))
			case *parse.UnaryNode:
				v = extract(e.walkUnary(t))
			case *parse.BinaryNode:
				v = extract(e.walkBinary(t))
			case *parse.ExprNode:
				v = e.walkExpr(t)
			case *parse.PrefixNode:
				v = extract(e.walkPrefix(t))
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
				v = FromScalar(f)
			}
			in = append(in, reflect.ValueOf(v))
		}

		f := reflect.ValueOf(node.F.F)
		fr := []reflect.Value{}

		if node.F.PrefixEnabled {
			if !node.F.PrefixKey {
				fr = f.Call(append([]reflect.Value{reflect.ValueOf("default"), reflect.ValueOf(e)}, in...))
			} else {
				fr = f.Call(append([]reflect.Value{reflect.ValueOf(node.Prefix), reflect.ValueOf(e)}, in...))
			}
		} else {
			fr = f.Call(append([]reflect.Value{reflect.ValueOf(e)}, in...))
		}

		res = fr[0].Interface().(*ResultSet)
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
func extract(res *ResultSet) interface{} {
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeScalar {
		return float64(res.Results[0].Value.Value().(Scalar))
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeESQuery {
		return res.Results[0].Value.Value()
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeAzureResourceList {
		return res.Results[0].Value.Value()
	}
	if len(res.Results) == 1 && res.Results[0].Type() == models.TypeAzureAIApps {
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

// CollectCacheHit is a helper function for collecting bosun metrics
// about the expression cache.
func CollectCacheHit(c *cache.Cache, qType string, hit bool) {
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
