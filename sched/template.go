package sched

import (
	"fmt"
	"io"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type context struct {
	*State
	Alert *conf.Alert

	context opentsdb.Context
}

func (s *Schedule) data(st *State, a *conf.Alert) interface{} {
	return &context{
		State:   st,
		Alert:   a,
		context: s.cache,
	}
}

func (s *Schedule) ExecuteBody(w io.Writer, a *conf.Alert, st *State) error {
	if a.Template == nil || a.Template.Body == nil {
		return nil
	}
	return a.Template.Body.Execute(w, s.data(st, a))
}

func (s *Schedule) ExecuteSubject(w io.Writer, a *conf.Alert, st *State) error {
	if a.Template == nil || a.Template.Subject == nil {
		return nil
	}
	return a.Template.Subject.Execute(w, s.data(st, a))
}

// E executes the given expression and returns a value with corresponding tags
// to the context's tags. If no such result is found, the first result with nil
// tags is returned. If no such result is found, nil is returned. The precision
// of numbers is truncated for convienent display. Array expressions are not
// supported.
func (c *context) E(v string) (s string) {
	e, err := expr.New(v)
	if err != nil {
		return
	}
	res, err := e.Execute(c.context, nil)
	if err != nil {
		return
	}
	for _, r := range res {
		if r.Group.Equal(c.State.Group) {
			s = truncate(r.Value)
		}
	}
	for _, r := range res {
		if r.Group == nil {
			s = truncate(r.Value)
		}
	}
	return
}

// truncate displays needed decimals for a Number.
func truncate(v expr.Value) string {
	switch t := v.(type) {
	case expr.Number:
		if t < 1 {
			return fmt.Sprintf("%.4f", t)
		} else if t < 100 {
			return fmt.Sprintf("%.1f", t)
		} else {
			return fmt.Sprintf("%.0f", t)
		}
	default:
		return ""
	}
}
