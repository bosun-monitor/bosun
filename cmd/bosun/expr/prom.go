package expr

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	promModels "github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql"
)

type PromConfig struct {
	URL string
}

// Prom is a map of functions to query Prometheus.
var Prom = map[string]parse.Func{
	"prom": {
		Args: []models.FuncType{
			models.TypeString, // query
			models.TypeString, // start
			models.TypeString, // end
			models.TypeString, // step
		},
		Return: models.TypeSeriesSet,
		Tags:   PromTag,
		F:      PromQuery,
	},
}

func PromTag(args []parse.Node) (parse.Tags, error) {
	st, err := promql.ParseMetricSelector(args[0].(*parse.StringNode).Text)
	if err != nil {
		return nil, err
	}
	t := make(parse.Tags)
	for _, s := range st {
		if string(s.Name) == "__name__" {
			continue
		}
		t[string(s.Name)] = struct{}{}
	}
	return t, nil
}

func PromQuery(e *State, query, startDuration, endDuration, stepDuration string) (*Results, error) {
	r := new(Results)
	sd, err := opentsdb.ParseDuration(startDuration)
	if err != nil {
		return nil, err
	}
	ed, err := opentsdb.ParseDuration(endDuration)
	if endDuration == "" {
		ed = 0
	} else if err != nil {
		return nil, err
	}
	start := time.Now().Add(-time.Duration(sd))
	end := time.Now().Add(-time.Duration(ed))
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return nil, err
	}
	step := time.Duration(st)
	qres, err := timePromRequest(e, query, start, end, step)
	if err != nil {
		return nil, err
	}
	for _, row := range qres.(promModels.Matrix) {
		tags := make(opentsdb.TagSet)
		for tagk, tagv := range row.Metric {
			tags[string(tagk)] = string(tagv)
		}
		if e.Squelched(tags) {
			continue
		}
		values := make(Series, len(row.Values))
		for _, v := range row.Values {
			values[v.Timestamp.Time()] = float64(v.Value)
		}
		r.Results = append(r.Results, &Result{
			Value: values,
			Group: tags,
		})
	}
	return r, nil
}

func timePromRequest(e *State, query string, start, end time.Time, step time.Duration) (s promModels.Value, err error) {
	//spew.Dump(os.Stderr, e)
	client, err := api.NewClient(api.Config{Address: e.PromConfig.URL})
	if err != nil {
		return nil, err
	}
	conn := v1.NewAPI(client)
	if err != nil {
		return nil, err
	}
	r := v1.Range{Start: start, End: end, Step: step}
	key := struct {
		Query string
		Range v1.Range
	}{
		query,
		r,
	}
	b, _ := json.MarshalIndent(key, "", "  ")
	e.Timer.StepCustomTiming("prom", "query", query, func() {
		getFn := func() (interface{}, error) {
			res, err := conn.QueryRange(context.Background(), query,
				r)
			if err != nil {
				return nil, err
			}
			m, ok := res.(promModels.Matrix)
			if !ok {
				return nil, fmt.Errorf("prom: expected matrix result")
			}
			return m, nil
		}
		var val interface{}
		var ok bool
		val, err, _ = e.Cache.Get(string(b), getFn)
		if s, ok = val.(promModels.Matrix); !ok {
			err = fmt.Errorf("prom: did not get valid result from prometheus, %v", err)
		}
	})
	return
}
