package expr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModels "github.com/prometheus/common/model"
)

// PromClients is a collection of Prometheus API v1 client APIs (connections)
type PromClients map[string]v1.API

// Prom is a map of functions to query Prometheus.
var Prom = map[string]parse.Func{
	"prom": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // groupby tags
			models.TypeString, // filter string
			models.TypeString, // aggregation type
			models.TypeString, // step interval duration
			models.TypeString, // StartDuration
			models.TypeString, // EndDuration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promGroupTags,
		F:             PromQuery,
		PrefixEnabled: true,
	},
}

func promGroupTags(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}

func PromQuery(prefix string, e *State, metric, groupBy, filter, agType, stepDuration, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	sd, err := opentsdb.ParseDuration(sdur)
	if err != nil {
		return
	}
	var ed opentsdb.Duration
	if edur != "" {
		ed, err = opentsdb.ParseDuration(edur)
		if err != nil {
			return
		}
	}
	start := e.now.Add(-time.Duration(sd))
	end := e.now.Add(-time.Duration(ed))
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return nil, err
	}
	step := time.Duration(st)
	qs := promQuery{
		Metric: metric,
		AgFunc: agType,
		Tags:   groupBy,
		Filter: filter,
	}
	buf := new(bytes.Buffer)
	err = promQueryTemplate.Execute(buf, qs)
	if err != nil {
		return
	}
	query := buf.String()
	qres, err := timePromRequest(e, prefix, query, start, end, step)
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

var promQueryTemplate = template.Must(template.New("promQueryTemplate").Parse(`
{{ .AgFunc }}( {{ .Metric -}} 
{{- if ne .Filter "" }} {{ .Filter | printf "{%v} " }} {{ end -}}
) by ( {{ .Tags }} )`))

type promQuery struct {
	Metric string
	AgFunc string
	Tags   string
	Filter string
}

func timePromRequest(e *State, prefix, query string, start, end time.Time, step time.Duration) (s promModels.Value, err error) {
	client, found := e.PromConfig[prefix]
	if !found {
		return s, fmt.Errorf(`prometheus client with name "%v" not defined`, prefix)
	}
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
			res, err := client.QueryRange(context.Background(), query,
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
		val, err, _ = e.Cache.Get(fmt.Sprintf("%v:%v", prefix, string(b)), getFn)
		if s, ok = val.(promModels.Matrix); !ok {
			err = fmt.Errorf("prom: did not get valid result from prometheus, %v", err)
		}
	})
	return
}