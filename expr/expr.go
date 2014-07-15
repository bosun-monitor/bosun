package expr

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/expr/parse"
)

type state struct {
	*Expr
	now     time.Time
	autods  int
	context opentsdb.Context
	queries []opentsdb.Request
}

func (e *state) addRequest(r opentsdb.Request) {
	e.queries = append(e.queries, r)
}

var ErrUnknownOp = fmt.Errorf("expr: unknown op type")

type Expr struct {
	*parse.Tree
}

func (e *Expr) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func New(expr string) (*Expr, error) {
	t, err := parse.Parse(expr, Builtins)
	if err != nil {
		return nil, err
	}
	e := &Expr{
		Tree: t,
	}
	return e, nil
}

// Execute applies a parse expression to the specified OpenTSDB context,
// and returns one result per group. T may be nil to ignore timings.
func (e *Expr) Execute(c opentsdb.Context, T miniprofiler.Timer) (r []*Result, queries []opentsdb.Request, err error) {
	return e.ExecuteOpts(c, T, time.Now().UTC(), 0)
}

// ExecuteOpts is identical to Execute, supports time setting and auto downsampling.
func (e *Expr) ExecuteOpts(c opentsdb.Context, T miniprofiler.Timer, now time.Time, autods int) (r []*Result, queries []opentsdb.Request, err error) {
	defer errRecover(&err)
	s := &state{
		Expr:    e,
		context: c,
		now:     now,
		autods:  autods,
	}
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

func (n Number) Type() parse.FuncType         { return parse.TYPE_NUMBER }
func (n Number) Value() interface{}           { return n }
func (n Number) MarshalJSON() ([]byte, error) { return marshalFloat(float64(n)) }

type Scalar float64

func (s Scalar) Type() parse.FuncType         { return parse.TYPE_SCALAR }
func (s Scalar) Value() interface{}           { return s }
func (s Scalar) MarshalJSON() ([]byte, error) { return marshalFloat(float64(s)) }

type Series map[string]opentsdb.Point

func (s Series) Type() parse.FuncType { return parse.TYPE_SERIES }
func (s Series) Value() interface{}   { return s }

type Result struct {
	Computations
	Value
	Group opentsdb.TagSet
}

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
func wrap(v float64) []*Result {
	return []*Result{
		{
			Value: Scalar(v),
			Group: nil,
		},
	}
}

func (u *Union) ExtendComputations(o *Result) {
	u.Computations = append(u.Computations, o.Computations...)
}

const unjoinedGroup = "unjoined group"

// union returns the combination of a and b where one is a strict subset of the
// other.
func union(a, b []*Result, expression string) []*Union {
	var us []*Union
	am := make(map[*Result]bool)
	bm := make(map[*Result]bool)
	for _, ra := range a {
		am[ra] = true
	}
	for _, rb := range b {
		bm[rb] = true
	}
	for _, ra := range a {
		for _, rb := range b {
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
	for r := range am {
		u := &Union{
			A:     r.Value,
			B:     Scalar(math.NaN()),
			Group: r.Group,
		}
		r.AddComputation(expression, unjoinedGroup)
		u.ExtendComputations(r)
		us = append(us, u)
	}
	for r := range bm {
		u := &Union{
			A:     Scalar(math.NaN()),
			B:     r.Value,
			Group: r.Group,
		}
		r.AddComputation(expression, unjoinedGroup)
		u.ExtendComputations(r)
		us = append(us, u)
	}
	return us
}

func (e *state) walk(node parse.Node, T miniprofiler.Timer) []*Result {
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

func (e *state) walkBinary(node *parse.BinaryNode, T miniprofiler.Timer) []*Result {
	ar := e.walk(node.Args[0], T)
	br := e.walk(node.Args[1], T)
	var res []*Result
	u := union(ar, br, node.String())
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
						s[k] = opentsdb.Point(operate(node.OpStr, float64(at), float64(v)))
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
						s[k] = opentsdb.Point(operate(node.OpStr, float64(at), float64(v)))
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
						s[k] = opentsdb.Point(operate(node.OpStr, float64(v), bv))
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
		res = append(res, &r)
	}
	return res
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

func (e *state) walkUnary(node *parse.UnaryNode, T miniprofiler.Timer) []*Result {
	a := e.walk(node.Arg, T)
	for _, r := range a {
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
				s[k] = opentsdb.Point(uoperate(node.OpStr, float64(v)))
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

func (e *state) walkFunc(node *parse.FuncNode, T miniprofiler.Timer) []*Result {
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
	res := fr[0].Interface().([]*Result)
	if len(fr) > 1 && !fr[1].IsNil() {
		err := fr[1].Interface().(error)
		if err != nil {
			panic(err)
		}
	}
	if node.Return() == parse.TYPE_NUMBER {
		for _, r := range res {
			r.AddComputation(node.String(), r.Value.(Number))
		}
	}
	return res
}

// extractScalar will return a float64 if res contains exactly one scalar.
func extractScalar(res []*Result) interface{} {
	if len(res) == 1 && res[0].Type() == parse.TYPE_SCALAR {
		return float64(res[0].Value.Value().(Scalar))
	}
	return res
}
