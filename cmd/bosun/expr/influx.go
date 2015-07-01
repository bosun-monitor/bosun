package expr

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/opentsdb"
	"github.com/influxdb/influxdb/client"
	"github.com/influxdb/influxdb/influxql"
)

const influxTimeFmt = "2006-01-02 15:04:05"

// Influx is a map of functions to query InfluxDB.
var Influx = map[string]parse.Func{
	"influx": {
		Args:   []parse.FuncType{parse.TypeString, parse.TypeString, parse.TypeString, parse.TypeString, parse.TypeString},
		Return: parse.TypeSeriesSet,
		Tags:   influxTag,
		F:      InfluxQuery,
	},
}

func influxTag(args []parse.Node) (parse.Tags, error) {
	n := args[4].(*parse.StringNode)
	t := make(parse.Tags)
	for _, k := range strings.Split(n.Text, ",") {
		t[k] = struct{}{}
	}
	return t, nil
}

func InfluxQuery(e *State, T miniprofiler.Timer, db, query, startDuration, endDuration, tagFormat string) (*Results, error) {
	qres, err := timeInfluxRequest(e, T, db, query, startDuration, endDuration)
	if err != nil {
		return nil, err
	}
	r := new(Results)
	expectTags := make(map[string]bool)
	for _, t := range strings.Split(tagFormat, ",") {
		expectTags[t] = true
	}
Loop:
	for _, row := range qres {
		tags := opentsdb.TagSet(row.Tags)
		for k, v := range tags {
			if v == "" {
				delete(tags, k)
			} else if !expectTags[k] {
				continue Loop
			}
		}
		if len(expectTags) != len(tags) {
			continue
		}
		if e.squelched(tags) {
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
func influxQueryDuration(now time.Time, query, start, end string) (string, error) {
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
	var cond string
	if s.Condition != nil {
		cond = s.Condition.String() + " and "
	}
	cond += fmt.Sprintf("time >= '%v' and time <= '%v'", now.Add(time.Duration(-sd)).Format(influxTimeFmt), now.Add(time.Duration(-ed)).Format(influxTimeFmt))
	e, err := influxql.ParseExpr(cond)
	if err != nil {
		return "", err
	}
	s.Condition = e
	return s.String(), nil
}

func timeInfluxRequest(e *State, T miniprofiler.Timer, db, query, startDuration, endDuration string) (s []influxql.Row, err error) {
	q, err := influxQueryDuration(e.now, query, startDuration, endDuration)
	if err != nil {
		return nil, err
	}
	conf := client.Config{
		URL: url.URL{
			Scheme: "http",
			Host:   e.InfluxHost,
		},
		Timeout: time.Minute,
	}
	conn, err := client.NewClient(conf)
	if err != nil {
		return nil, err
	}
	T.StepCustomTiming("influx", "query", q, func() {
		getFn := func() (interface{}, error) {
			res, err := conn.Query(client.Query{
				Command:  q,
				Database: db,
			})
			if err != nil {
				return nil, err
			}
			if res.Err != nil {
				return nil, res.Err
			}
			if len(res.Results) != 1 {
				return nil, fmt.Errorf("influx: expected one result")
			}
			r := res.Results[0]
			fmt.Println("GOT", len(r.Series))
			return r.Series, r.Err
		}
		var val interface{}
		val, err = e.cache.Get(q, getFn)
		s = val.([]influxql.Row)
	})
	return
}
