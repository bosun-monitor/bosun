package expr // import "bosun.org/cmd/bosun/expr"

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"sort"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/cmd/bosun/search"
	"bosun.org/graphite"
	"bosun.org/opentsdb"
)

type State struct {
	*Expr
	now time.Time

	// openTSDB specific
	Search     *search.Search
	autods     int
	context    opentsdb.Context
	queries    []opentsdb.Request
	unjoinedOk bool
	squelched  func(tags opentsdb.TagSet) bool

	// graphite specific
	graphiteQueries []graphite.Request
	graphiteContext graphite.Context
}

func (e *State) addRequest(r interface{}) {
	switch r.(type) {
	case opentsdb.Request:
		e.queries = append(e.queries, r.(opentsdb.Request))
	case graphite.Request:
		e.graphiteQueries = append(e.graphiteQueries, r.(graphite.Request))
	}

}

var ErrUnknownOp = fmt.Errorf("expr: unknown op type")

type Expr struct {
	*parse.Tree
}

func (e *Expr) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

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
func (e *Expr) Execute(c opentsdb.Context, g graphite.Context, T miniprofiler.Timer, now time.Time, autods int, unjoinedOk bool, search *search.Search, squelched func(tags opentsdb.TagSet) bool) (r *Results, queries []opentsdb.Request, err error) {
	if squelched == nil {
		squelched = func(tags opentsdb.TagSet) bool {
			return false
		}
	}
	s := &State{
		Expr:            e,
		context:         c,
		graphiteContext: g,
		now:             now,
		autods:          autods,
		unjoinedOk:      unjoinedOk,
		Search:          search,
		squelched:       squelched,
	}
	return e.ExecuteState(s, T)
}

func (e *Expr) ExecuteState(s *State, T miniprofiler.Timer) (r *Results, queries []opentsdb.Request, err error) {
	defer errRecover(&err)
	if T == nil {
		T = new(miniprofiler.Profile)
	}
	T.Step("expr execute", func(T miniprofiler.Timer) {
		r = s.walk(e.Tree.Root, T)
	})
	queries = s.queries
	return
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = err
		default:
			panic(e)
		}
	}
}

type Value interface {
	Type() parse.FuncType
	Value() interface{}
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

type Number float64

func (n Number) Type() parse.FuncType { return parse.TypeNumber }
func (n Number) Value() interface{}   { return n }

//func (n Number) MarshalJSON() ([]byte, error) { return marshalFloat(float64(n)) }

type Scalar float64

func (s Scalar) Type() parse.FuncType { return parse.TypeScalar }
func (s Scalar) Value() interface{}   { return s }

//func (s Scalar) MarshalJSON() ([]byte, error) { return marshalFloat(float64(s)) }

type Series map[time.Time]float64

func (s Series) Type() parse.FuncType { return parse.TypeSeries }
func (s Series) Value() interface{}   { return s }

type Result struct {
	Computations
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

type ResultSlice []*Result

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
	sort.Sort(c)
	return c
}

func (r ResultSlice) Len() int           { return len(r) }
func (r ResultSlice) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ResultSlice) Less(i, j int) bool { return r[i].Value.(Number) > r[j].Value.(Number) }

type Computations []Computation

type Computation struct {
	Text  string
	Value interface{}
}

func (r *Result) AddComputation(text string, value interface{}) {
	r.Computations = append(r.Computations, Computation{opentsdb.ReplaceTags(text, r.Group), value})
}

type Union struct {
	Computations
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
	am := make(map[*Result]bool)
	bm := make(map[*Result]bool)
	for _, ra := range a.Results {
		am[ra] = true
	}
	for _, rb := range b.Results {
		bm[rb] = true
	}
	for _, ra := range a.Results {
		for _, rb := range b.Results {
			u := &Union{
				A: ra.Value,
				B: rb.Value,
			}
			if ra.Group.Equal(rb.Group) || len(ra.Group) == 0 || len(rb.Group) == 0 {
				g := ra.Group
				if len(ra.Group) == 0 {
					g = rb.Group
				}
				u.Group = g
			} else if ra.Group.Subset(rb.Group) {
				u.Group = ra.Group
			} else if rb.Group.Subset(ra.Group) {
				u.Group = rb.Group
			} else {
				continue
			}
			delete(am, ra)
			delete(bm, rb)
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
				r.AddComputation(expression, fmt.Sprintf(unjoinedGroup, u.B))
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
				r.AddComputation(expression, fmt.Sprintf(unjoinedGroup, u.A))
				u.ExtendComputations(r)
				us = append(us, u)
			}
		}
	}
	return us
}

func (e *State) walk(node parse.Node, T miniprofiler.Timer) *Results {
	switch node := node.(type) {
	case *parse.NumberNode:
		return wrap(node.Float64)
	case *parse.BinaryNode:
		return e.walkBinary(node, T)
	case *parse.UnaryNode:
		return e.walkUnary(node, T)
	case *parse.FuncNode:
		return e.walkFunc(node, T)
	default:
		panic(fmt.Errorf("expr: unknown node type"))
	}
}

func (e *State) walkBinary(node *parse.BinaryNode, T miniprofiler.Timer) *Results {
	ar := e.walk(node.Args[0], T)
	br := e.walk(node.Args[1], T)
	res := Results{
		IgnoreUnjoined:      ar.IgnoreUnjoined || br.IgnoreUnjoined,
		IgnoreOtherUnjoined: ar.IgnoreOtherUnjoined || br.IgnoreOtherUnjoined,
	}
	u := e.union(ar, br, node.String())
	for _, v := range u {
		var value Value
		r := Result{
			Group:        v.Group,
			Computations: v.Computations,
		}
		an, aok := v.A.(Scalar)
		bn, bok := v.B.(Scalar)
		if (aok && math.IsNaN(float64(an))) || (bok && math.IsNaN(float64(bn))) {
			value = Scalar(math.NaN())
		} else {
			switch at := v.A.(type) {
			case Scalar:
				switch bt := v.B.(type) {
				case Scalar:
					n := Scalar(operate(node.OpStr, float64(at), float64(bt)))
					r.AddComputation(node.String(), Number(n))
					value = n
				case Number:
					n := Number(operate(node.OpStr, float64(at), float64(bt)))
					r.AddComputation(node.String(), n)
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
					r.AddComputation(node.String(), Number(n))
					value = n
				case Number:
					n := Number(operate(node.OpStr, float64(at), float64(bt)))
					r.AddComputation(node.String(), n)
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
				default:
					panic(ErrUnknownOp)
				}
			default:
				panic(ErrUnknownOp)
			}
		}
		r.Value = value
		res.Results = append(res.Results, &r)
	}
	return &res
}

func operate(op string, a, b float64) (r float64) {
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

func (e *State) walkFunc(node *parse.FuncNode, T miniprofiler.Timer) *Results {
	f := reflect.ValueOf(node.F.F)
	var in []reflect.Value
	for _, a := range node.Args {
		var v interface{}
		switch t := a.(type) {
		case *parse.StringNode:
			v = t.Text
		case *parse.NumberNode:
			v = t.Float64
		case *parse.FuncNode:
			v = extractScalar(e.walkFunc(t, T))
		case *parse.UnaryNode:
			v = extractScalar(e.walkUnary(t, T))
		case *parse.BinaryNode:
			v = extractScalar(e.walkBinary(t, T))
		default:
			panic(fmt.Errorf("expr: unknown func arg type"))
		}
		in = append(in, reflect.ValueOf(v))
	}
	fr := f.Call(append([]reflect.Value{reflect.ValueOf(e), reflect.ValueOf(T)}, in...))
	res := fr[0].Interface().(*Results)
	if len(fr) > 1 && !fr[1].IsNil() {
		err := fr[1].Interface().(error)
		if err != nil {
			panic(err)
		}
	}
	if node.Return() == parse.TypeNumber {
		for _, r := range res.Results {
			r.AddComputation(node.String(), r.Value.(Number))
		}
	}
	return res
}

// extractScalar will return a float64 if res contains exactly one scalar.
func extractScalar(res *Results) interface{} {
	if len(res.Results) == 1 && res.Results[0].Type() == parse.TypeScalar {
		return float64(res.Results[0].Value.Value().(Scalar))
	}
	return res
}
