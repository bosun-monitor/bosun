package expr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/prometheus/promql"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModels "github.com/prometheus/common/model"
)

// PromClients is a collection of Prometheus API v1 client APIs (connections)
type PromClients map[string]promv1.API

// Prom is a map of functions to query Prometheus.
var Prom = map[string]parse.Func{
	"prom": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // groupby tags
			models.TypeString, // filter string
			models.TypeString, // aggregation type
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promGroupTags,
		F:             PromQuery,
		PrefixEnabled: true,
	},
	"promm": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // groupby tags
			models.TypeString, // filter string
			models.TypeString, // aggregation type
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promMGroupTags,
		F:             PromMQuery,
		PrefixEnabled: true,
	},
	"promrate": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // groupby tags
			models.TypeString, // filter string
			models.TypeString, // aggregation type
			models.TypeString, // rate step interval duration
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promGroupTags,
		F:             PromRate,
		PrefixEnabled: true,
	},
	"promratem": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // groupby tags
			models.TypeString, // filter string
			models.TypeString, // aggregation type
			models.TypeString, // rate step interval duration
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promMGroupTags,
		F:             PromMRate,
		PrefixEnabled: true,
	},
	"promras": { // prom raw aggregated series
		Args: []models.FuncType{
			models.TypeString, // promql query
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		Tags:          promAggregateRawTags,
		F:             PromRawAggregateSeriesQuery,
		PrefixEnabled: true,
	},
	"prommetrics": {
		Args:          []models.FuncType{},
		Return:        models.TypeInfo,
		F:             PromMetricList,
		PrefixEnabled: true,
	},
	"promtags": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeInfo,
		F:             PromTagInfo,
		PrefixEnabled: true,
	},
}

// promGroupTags parses the csv tags argument of the prom based functions
func promGroupTags(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}

// promMGroupTags parses the csv tags argument of the prom based functions
// and also adds the "bosun_prefix" tag
func promMGroupTags(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	tags["bosun_prefix"] = struct{}{}
	return tags, nil
}

// promAggregateRawTags parses the promql argument to get the expected
// grouping tags from an aggregated series
func promAggregateRawTags(args []parse.Node) (parse.Tags, error) {
	tags := make(parse.Tags)
	pq := args[0].(*parse.StringNode).Text
	parsedPromExpr, err := promql.ParseExpr(pq)
	if err != nil {
		return nil, fmt.Errorf("failed to extract tags from promql query due to invalid promql expression: %v", err)
	}
	promAgExprNode, ok := parsedPromExpr.(*promql.AggregateExpr)
	if !ok || promAgExprNode == nil {
		return nil, fmt.Errorf("failed to extract tags from promql query, top level expression is not aggregation operation: %v", err)
	}
	for _, k := range promAgExprNode.Grouping {
		tags[k] = struct{}{}
	}
	return tags, nil
}

func PromRawAggregateSeriesQuery(prefix string, e *State, query, stepDuration, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	parsedPromExpr, err := promql.ParseExpr(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse invalid promql expression: %v", err)
	}
	promAgExprNode, ok := parsedPromExpr.(*promql.AggregateExpr)
	if !ok || promAgExprNode == nil {
		return nil, fmt.Errorf("top level expression is not aggregation operation")
	}
	start, end, err := parseDurationPair(e, sdur, edur)
	if err != nil {
		return
	}
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return
	}
	step := time.Duration(st)
	tagLen := len(promAgExprNode.Grouping)
	qRes, err := timePromRequest(e, prefix, query, start, end, step)
	if err != nil {
		return nil, err
	}
	for _, row := range qRes.(promModels.Matrix) {
		tags := make(opentsdb.TagSet)
		for tagK, tagV := range row.Metric {
			tags[string(tagK)] = string(tagV)
		}
		// Remove results with less tag keys than those requests
		if len(tags) < tagLen {
			continue
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
	return
}

// PromMetricList returns a list of available metrics for the prometheus backend
// by using querying the Prometheus Lable Values API for "__name__"
func PromMetricList(prefix string, e *State) (r *Results, err error) {
	r = new(Results)
	client, found := e.PromConfig[prefix]
	if !found {
		return r, fmt.Errorf(`prometheus client with name "%v" not defined`, prefix)
	}
	getFn := func() (interface{}, error) {
		var metrics promModels.LabelValues
		e.Timer.StepCustomTiming("prom", "metriclist", "", func() {
			metrics, err = client.LabelValues(context.Background(), "__name__")
		})
		if err != nil {
			return nil, err
		}
		return metrics, nil
	}
	val, err, hit := e.Cache.Get(fmt.Sprintf("%v:metriclist", prefix), getFn)
	collectCacheHit(e.Cache, "prom_metrics", hit)
	if err != nil {
		return nil, err
	}
	metrics := val.(promModels.LabelValues)
	r.Results = append(r.Results, &Result{Value: Info{metrics}})
	return
}

// PromTagInfo does a range query for the given metric and returns info about the
// tags and labels for the metric based on the data from the queried timeframe
func PromTagInfo(prefix string, e *State, metric, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	client, found := e.PromConfig[prefix]
	if !found {
		return r, fmt.Errorf(`prometheus client with name "%v" not defined`, prefix)
	}
	start, end, err := parseDurationPair(e, sdur, edur)
	if err != nil {
		return
	}

	qRange := promv1.Range{Start: start, End: end, Step: time.Minute}

	getFn := func() (interface{}, error) {
		var res promModels.Value
		e.Timer.StepCustomTiming("prom", "taginfo", metric, func() {
			res, err = client.QueryRange(context.Background(), metric, qRange)
		})
		if err != nil {
			return nil, err
		}
		m, ok := res.(promModels.Matrix)
		if !ok {
			return nil, fmt.Errorf("prom: expected a prometheus matrix type in result but got %v", res.Type().String())
		}
		return m, nil
	}
	val, err, hit := e.Cache.Get(fmt.Sprintf("%v:%v:taginfo", prefix, metric), getFn)
	collectCacheHit(e.Cache, "prom_metrics", hit)
	if err != nil {
		return nil, err
	}
	matrix, ok := val.(promModels.Matrix)
	if !ok {
		err = fmt.Errorf("prom: did not get valid result from prometheus, %v", err)
	}
	tagInfo := struct {
		Metric       string
		Keys         []string
		KeysToValues map[string][]string
		UniqueSets   []string
	}{}
	tagInfo.Metric = metric
	tagInfo.KeysToValues = make(map[string][]string)
	sets := make(map[string]struct{})
	keysToValues := make(map[string]map[string]struct{})
	for _, row := range matrix {
		tags := make(opentsdb.TagSet)
		for rawTagK, rawTagV := range row.Metric {
			tagK := string(rawTagK)
			tagV := string(rawTagV)
			if tagK == "__name__" {
				continue
			}
			tags[tagK] = tagV
			if _, ok := keysToValues[tagK]; !ok {
				keysToValues[tagK] = make(map[string]struct{})
			}
			keysToValues[tagK][tagV] = struct{}{}
		}
		sets[tags.String()] = struct{}{}
	}
	for k, values := range keysToValues {
		tagInfo.Keys = append(tagInfo.Keys, k)
		for val := range values {
			tagInfo.KeysToValues[k] = append(tagInfo.KeysToValues[k], val)
		}
	}
	sort.Strings(tagInfo.Keys)
	for s := range sets {
		tagInfo.UniqueSets = append(tagInfo.UniqueSets, s)
	}
	sort.Strings(tagInfo.UniqueSets)
	r.Results = append(r.Results, &Result{
		Value: Info{tagInfo},
	})
	return
}

// PromQuery is a wrapper for promQuery so there is a function signature that doesn't require the rate argument in the expr language.
// It also sets promQuery's addPrefixTag argument to false since this only queries one backend.
func PromQuery(prefix string, e *State, metric, groupBy, filter, agType, stepDuration, sdur, edur string) (r *Results, err error) {
	return promQuery(prefix, e, metric, groupBy, filter, agType, "", stepDuration, sdur, edur, false)
}

// PromRate is a wrapper for promQuery like PromQuery except that it has a rateDuration argument for the step of the rate calculation.
// This enables rate calculation for counters.
func PromRate(prefix string, e *State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *Results, err error) {
	return promQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, false)
}

// PromMQuery is a wrapper from promMQuery in the way that PromQuery is a wrapper from promQuery.
func PromMQuery(prefix string, e *State, metric, groupBy, filter, agType, stepDuration, sdur, edur string) (r *Results, err error) {
	return promMQuery(prefix, e, metric, groupBy, filter, agType, "", stepDuration, sdur, edur)
}

// PromMRate is a wrapper from promMQuery in the way that PromRate is a wrapper from promQuery. It has a stepDuration argument
// for rate calculation.
func PromMRate(prefix string, e *State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *Results, err error) {
	return promMQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur)
}

// promMQuery makes call to multiple prometheus TSDBs and combines the results into a single series set.
// It adds the "bosun_prefix" tag key with the value of prefix label to the results. Queries are executed in parallel.
func promMQuery(prefix string, e *State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	prefixes := strings.Split(prefix, ",")
	if len(prefixes) == 1 && prefixes[0] == "" {
		return promQuery("default", e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, true)
	}

	wg := sync.WaitGroup{}
	wg.Add(len(prefixes))
	resCh := make(chan *Results, len(prefixes))
	errCh := make(chan error, len(prefixes))

	for _, prefix := range prefixes {
		go func(prefix string) {
			defer wg.Done()
			res, err := promQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, true)
			resCh <- res
			errCh <- err
		}(prefix)
	}

	wg.Wait()
	close(resCh)
	close(errCh)
	// Gather errors from the request and return an error if any of the requests failled
	errors := []string{}
	for err := range errCh {
		if err == nil {
			continue
		}
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		return r, fmt.Errorf(strings.Join(errors, " :: "))
	}
	resultCollection := []*Results{}
	for res := range resCh {
		resultCollection = append(resultCollection, res)
	}
	if len(resultCollection) == 1 { // no need to merge if there is only one item
		return resultCollection[0], nil
	}
	// Merge the query results into a single seriesSet
	r, err = Merge(e, resultCollection...)
	return
}

// promQuery uses the information passed to it to generate an PromQL query using the promQueryTemplate.
// It then calls timePromRequest to execute the query and process that results in to a Bosun Results object.
func promQuery(prefix string, e *State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string, addPrefixTag bool) (r *Results, err error) {
	r = new(Results)
	start, end, err := parseDurationPair(e, sdur, edur)
	if err != nil {
		return
	}
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return
	}
	step := time.Duration(st)
	qd := promQueryTemplateData{
		Metric:       metric,
		AgFunc:       agType,
		Tags:         groupBy,
		Filter:       filter,
		RateDuration: rateDuration,
	}
	query, err := qd.RenderString()
	qRes, err := timePromRequest(e, prefix, query, start, end, step)
	if err != nil {
		return
	}

	groupByTagSet := make(opentsdb.TagSet)
	for _, v := range strings.Split(groupBy, ",") {
		if v != "" {
			groupByTagSet[v] = ""
		}
	}
	for _, row := range qRes.(promModels.Matrix) {
		tags := make(opentsdb.TagSet)
		for tagK, tagV := range row.Metric {
			tags[string(tagK)] = string(tagV)
		}
		// Remove results with less tag keys than those requests
		if len(tags) < len(groupByTagSet) {
			continue
		}
		if addPrefixTag {
			tags["bosun_prefix"] = prefix
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

// promQueryTemplate is a template for PromQL time series queries. It supports
// filtering and aggregation
var promQueryTemplate = template.Must(template.New("promQueryTemplate").Parse(`
{{ .AgFunc }}(
{{- if ne .RateDuration "" }}rate({{ end }} {{ .Metric -}}
{{- if ne .Filter "" }} {{ .Filter | printf "{%v} " -}} {{- end -}}
{{- if ne .RateDuration "" -}} {{ .RateDuration | printf " [%v] )"  }} {{- end -}}
) by ( {{ .Tags }} )`))

// promQueryTemplateData is the struct the contains the fields to render the promQueryTemplate
type promQueryTemplateData struct {
	Metric       string
	AgFunc       string
	Tags         string
	Filter       string
	RateDuration string
}

// RenderString creates a query string using promQueryTemplate
func (pq promQueryTemplateData) RenderString() (string, error) {
	buf := new(bytes.Buffer)
	err := promQueryTemplate.Execute(buf, pq)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// timePromRequest takes a PromQL query string with the given time frame and step duration. The result
// type of the PromQL query must be a Prometheus Matrix
func timePromRequest(e *State, prefix, query string, start, end time.Time, step time.Duration) (s promModels.Value, err error) {
	client, found := e.PromConfig[prefix]
	if !found {
		return s, fmt.Errorf(`prometheus client with name "%v" not defined`, prefix)
	}
	r := promv1.Range{Start: start, End: end, Step: step}
	cacheKey := struct {
		Query  string
		Range  promv1.Range
		Step   time.Duration
		Prefix string
	}{
		query,
		r,
		step,
		prefix,
	}
	cacheKeyBytes, _ := json.MarshalIndent(cacheKey, "", "  ")
	e.Timer.StepCustomTiming("prom", fmt.Sprintf("query (%v)", prefix), query, func() {
		getFn := func() (interface{}, error) {
			res, err := client.QueryRange(context.Background(), query, r)
			if err != nil {
				return nil, err
			}
			m, ok := res.(promModels.Matrix)
			if !ok {
				return nil, fmt.Errorf("prom: expected matrix result")
			}
			return m, nil
		}
		val, err, hit := e.Cache.Get(string(cacheKeyBytes), getFn)
		collectCacheHit(e.Cache, "prom_ts", hit)
		var ok bool
		if s, ok = val.(promModels.Matrix); !ok {
			err = fmt.Errorf("prom: did not get valid result from prometheus, %v", err)
		}
	})
	return
}
