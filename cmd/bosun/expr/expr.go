package expr // import "bosun.org/cmd/bosun/expr"

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"runtime/debug"
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
func (e *Expr) Execute(tsdbs *TSDBs, providers *BosunProviders, T miniprofiler.Timer, now time.Time, autods int, unjoinedOk bool, origin string) (r *ValueSet, queries []opentsdb.Request, err error) {
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
func (e *Expr) ExecuteState(s *State) (r *ValueSet, queries []opentsdb.Request, err error) {
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

// AddComputation adds a computation to the State if the state has the enabledComputations
// property enabled.
// If the computation has OpenTSDB style tags each individual computation will correctly represent the specific
// tag value for the computation. Otherwise it will be the raw text part of the expression that represents the
// computation as inputted.
func (e *State) AddComputation(r *Element, text string, value interface{}) {
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
func wrap(v float64) *ValueSet {
	return &ValueSet{
		Elements: []*Element{
			{
				Value: Scalar(v),
				Group: nil,
			},
		},
	}
}

func (u *Union) ExtendComputations(o *Element) {
	u.Computations = append(u.Computations, o.Computations...)
}

// NaN returns the specified substitue value for NaN on the if one is present as a property on
// the ResultSet, else "NaN" is returned.
// The NaNValue property of the ResultSet is set when the nv() function is used in the expression language.
func (r *ValueSet) NaN() Number {
	if r.NaNValue != nil {
		return Number(*r.NaNValue)
	}
	return Number(math.NaN())
}

// union returns the combination of a and b where one is a subset of the other.
func (e *State) union(a, b *ValueSet, expression string) []*Union {
	const unjoinedGroup = "unjoined group (%v)"
	var us []*Union
	if len(a.Elements) == 0 || len(b.Elements) == 0 {
		return us
	}
	am := make(map[*Element]bool, len(a.Elements))
	bm := make(map[*Element]bool, len(b.Elements))
	for _, ra := range a.Elements {
		am[ra] = true
	}
	for _, rb := range b.Elements {
		bm[rb] = true
	}
	var group opentsdb.TagSet
	for _, ra := range a.Elements {
		for _, rb := range b.Elements {
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

func (e *State) walk(node parse.Node) *ValueSet {
	var res *ValueSet
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

func (e *State) walkExpr(node *parse.ExprNode) *ValueSet {
	return &ValueSet{
		Elements: ElementSlice{
			&Element{
				Value: NumberExpr{node.Tree},
			},
		},
	}
}

func (e *State) walkBinary(node *parse.BinaryNode) *ValueSet {
	ar := e.walk(node.Args[0])
	br := e.walk(node.Args[1])
	res := ValueSet{
		IgnoreUnjoined:      ar.IgnoreUnjoined || br.IgnoreUnjoined,
		IgnoreOtherUnjoined: ar.IgnoreOtherUnjoined || br.IgnoreOtherUnjoined,
	}
	e.Timer.Step("walkBinary: "+node.OpStr, func(T miniprofiler.Timer) {
		u := e.union(ar, br, node.String())
		for _, v := range u {
			var value Value
			r := &Element{
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
			res.Elements = append(res.Elements, r)
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

func (e *State) walkUnary(node *parse.UnaryNode) *ValueSet {
	a := e.walk(node.Arg)
	e.Timer.Step("walkUnary: "+node.OpStr, func(T miniprofiler.Timer) {
		for _, r := range a.Elements {
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

func (e *State) walkPrefix(node *parse.PrefixNode) *ValueSet {
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

func (e *State) walkFunc(node *parse.FuncNode) *ValueSet {
	var res *ValueSet
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

		res = fr[0].Interface().(*ValueSet)
		if len(fr) > 1 && !fr[1].IsNil() {
			err := fr[1].Interface().(error)
			if err != nil {
				panic(err)
			}
		}
		if node.Return() == models.TypeNumberSet {
			for _, r := range res.Elements {
				e.AddComputation(r, node.String(), r.Value.(Number))
			}
		}
	})
	return res
}

// extract will return a float64 if res contains exactly one scalar or a ESQuery if that is the type
func extract(res *ValueSet) interface{} {
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeScalar {
		return float64(res.Elements[0].Value.Value().(Scalar))
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeESQuery {
		return res.Elements[0].Value.Value()
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeAzureResourceList {
		return res.Elements[0].Value.Value()
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeAzureAIApps {
		return res.Elements[0].Value.Value()
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeESIndexer {
		return res.Elements[0].Value.Value()
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeString {
		return string(res.Elements[0].Value.Value().(String))
	}
	if len(res.Elements) == 1 && res.Elements[0].Type() == models.TypeNumberExpr {
		return res.Elements[0].Value.Value()
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
