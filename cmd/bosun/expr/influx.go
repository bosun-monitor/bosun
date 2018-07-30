package expr

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/influxql"
	influxModels "github.com/influxdata/influxdb/models"
)

// Influx is a map of functions to query InfluxDB.
var Influx = map[string]parse.Func{
	"influx": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   influxTag,
		F:      InfluxQuery,
	},
}

func influxTag(args []parse.Node) (parse.Tags, error) {
	st, err := influxql.ParseStatement(args[1].(*parse.StringNode).Text)
	if err != nil {
		return nil, err
	}
	s, ok := st.(*influxql.SelectStatement)
	if !ok {
		return nil, fmt.Errorf("influx: expected select statement")
	}

	t := make(parse.Tags, len(s.Dimensions))
	for _, d := range s.Dimensions {
		if _, ok := d.Expr.(*influxql.Call); ok {
			continue
		}
		t[d.String()] = struct{}{}
	}
	return t, nil
}

func InfluxQuery(e *State, T miniprofiler.Timer, db, query, startDuration, endDuration, groupByInterval string) (*Results, error) {
	qres, err := timeInfluxRequest(e, T, db, query, startDuration, endDuration, groupByInterval)
	if err != nil {
		return nil, err
	}
	r := new(Results)
	for _, row := range qres {
		tags := opentsdb.TagSet(row.Tags)
		if e.Squelched(tags) {
			continue
		}
		if len(row.Columns) != 2 {
			return nil, fmt.Errorf("influx: expected exactly one result column")
		}
		values := make(Series, len(row.Values))
		for _, v := range row.Values {
			if len(v) != 2 {
				return nil, fmt.Errorf("influx: expected exactly one result column")
			}
			ts, ok := v[0].(string)
			if !ok {
				return nil, fmt.Errorf("influx: expected time string column")
			}
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				return nil, err
			}
			n, ok := v[1].(json.Number)
			if !ok {
				return nil, fmt.Errorf("influx: expected json.Number")
			}
			f, err := n.Float64()
			if err != nil {
				return nil, fmt.Errorf("influx: bad number: %v", err)
			}
			values[t] = f
		}
		r.Results = append(r.Results, &Result{
			Value: values,
			Group: tags,
		})
	}
	_ = r
	return r, nil
}

// influxQueryDuration adds time WHERE clauses to query for the given start and end durations.
func influxQueryDuration(now time.Time, query, start, end, groupByInterval string) (string, error) {
	sd, err := opentsdb.ParseDuration(start)
	if err != nil {
		return "", err
	}
	ed, err := opentsdb.ParseDuration(end)
	if end == "" {
		ed = 0
	} else if err != nil {
		return "", err
	}
	st, err := influxql.ParseStatement(query)
	if err != nil {
		return "", err
	}
	s, ok := st.(*influxql.SelectStatement)
	if !ok {
		return "", fmt.Errorf("influx: expected select statement")
	}
	isTime := func(n influxql.Node) bool {
		v, ok := n.(*influxql.VarRef)
		if !ok {
			return false
		}
		s := strings.ToLower(v.Val)
		return s == "time"
	}
	influxql.WalkFunc(s.Condition, func(n influxql.Node) {
		b, ok := n.(*influxql.BinaryExpr)
		if !ok {
			return
		}
		if isTime(b.LHS) || isTime(b.RHS) {
			err = fmt.Errorf("influx query must not contain time in WHERE")
		}
	})
	if err != nil {
		return "", err
	}

	//Add New BinaryExpr for time clause
	startExpr := &influxql.BinaryExpr{
		Op:  influxql.GTE,
		LHS: &influxql.VarRef{Val: "time"},
		RHS: &influxql.TimeLiteral{Val: now.Add(time.Duration(-sd))},
	}

	stopExpr := &influxql.BinaryExpr{
		Op:  influxql.LTE,
		LHS: &influxql.VarRef{Val: "time"},
		RHS: &influxql.TimeLiteral{Val: now.Add(time.Duration(-ed))},
	}

	if s.Condition != nil {
		s.Condition = &influxql.BinaryExpr{
			Op:  influxql.AND,
			LHS: s.Condition,
			RHS: &influxql.BinaryExpr{
				Op:  influxql.AND,
				LHS: startExpr,
				RHS: stopExpr,
			},
		}
	} else {
		s.Condition = &influxql.BinaryExpr{
			Op:  influxql.AND,
			LHS: startExpr,
			RHS: stopExpr,
		}
	}

	// parse last argument
	if len(groupByInterval) > 0 {
		gbi, err := time.ParseDuration(groupByInterval)
		if err != nil {
			return "", err
		}
		s.Dimensions = append(s.Dimensions,
			&influxql.Dimension{Expr: &influxql.Call{
				Name: "time",
				Args: []influxql.Expr{&influxql.DurationLiteral{Val: gbi}},
			},
			})
	}

	// emtpy aggregate windows should be purged from the result
	// this default resembles the opentsdb results.
	if s.Fill == influxql.NullFill {
		s.Fill = influxql.NoFill
		s.FillValue = nil
	}

	return s.String(), nil
}

func timeInfluxRequest(e *State, T miniprofiler.Timer, db, query, startDuration, endDuration, groupByInterval string) (s []influxModels.Row, err error) {
	q, err := influxQueryDuration(e.now, query, startDuration, endDuration, groupByInterval)
	if err != nil {
		return nil, err
	}
	conn, err := client.NewHTTPClient(e.InfluxConfig)
	if err != nil {
		return nil, err
	}
	q_key := fmt.Sprintf("%s: %s", db, q)
	T.StepCustomTiming("influx", "query", q_key, func() {
		getFn := func() (interface{}, error) {
			res, err := conn.Query(client.Query{
				Command:  q,
				Database: db,
			})
			if err != nil {
				return nil, err
			}
			if res.Error() != nil {
				return nil, res.Error()
			}
			if len(res.Results) != 1 {
				return nil, fmt.Errorf("influx: expected one result")
			}

			r := res.Results[0]
			if r.Err == "" {
				return r.Series, nil
			}
			err = fmt.Errorf(r.Err)
			return r.Series, err
		}
		var val interface{}
		var ok bool
		var hit bool
		val, err, hit = e.Cache.Get(q_key, getFn)
		collectCacheHit(e.Cache, "influx", hit)
		if s, ok = val.([]influxModels.Row); !ok {
			err = fmt.Errorf("influx: did not get a valid result from InfluxDB")
		}
	})
	return
}
