// Package prom contains Prometheus query functions for the Bosun expression language.
package prom

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
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModels "github.com/prometheus/common/model"
)

// ExprFuncs defines Bosun expression functions for use with Prometheus backends.
var ExprFuncs = map[string]parse.Func{
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
		TagKeys:       groupTags,
		F:             Query,
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
		TagKeys:       mGroupTags,
		F:             MQuery,
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
		TagKeys:       groupTags,
		F:             Rate,
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
		TagKeys:       mGroupTags,
		F:             MRate,
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
		TagKeys:       aggRawTags,
		F:             RawAggSeriesQuery,
		PrefixEnabled: true,
	},
	"prommras": { // prom multi raw aggregated series
		Args: []models.FuncType{
			models.TypeString, // promql query
			models.TypeString, // step interval duration
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeSeriesSet,
		TagKeys:       mAggRawTags,
		F:             MRawAggSeriesQuery,
		PrefixEnabled: true,
	},
	"prommetrics": {
		Args:          []models.FuncType{},
		Return:        models.TypeInfo,
		F:             MetricList,
		PrefixEnabled: true,
	},
	"promtags": {
		Args: []models.FuncType{
			models.TypeString, // metric
			models.TypeString, // start duration
			models.TypeString, // end duration
		},
		Return:        models.TypeInfo,
		F:             TagInfo,
		PrefixEnabled: true,
	},
}

// multiKey is the value for the tag key that is added to multibackend queries.
const multiKey = "bosun_prefix"

// groupTags parses the csv tags argument of the prom based functions
func groupTags(args []parse.Node) (parse.TagKeys, error) {
	tags := make(parse.TagKeys)
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}

// mGroupTags parses the csv tags argument of the prom based functions
// and also adds the promMultiKey tag
func mGroupTags(args []parse.Node) (parse.TagKeys, error) {
	tags, err := groupTags(args)
	if err != nil {
		return nil, err
	}
	tags[multiKey] = struct{}{}
	return tags, nil
}

// aggRawTags parses the promql argument to get the expected
// grouping tags from an aggregated series
func aggRawTags(args []parse.Node) (parse.TagKeys, error) {
	tags := make(parse.TagKeys)
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

// mAggRawTags is a wrapper for aggRawTags but adds the multiKey tag.
func mAggRawTags(args []parse.Node) (parse.TagKeys, error) {
	tags, err := aggRawTags(args)
	if err != nil {
		return nil, err
	}
	tags[multiKey] = struct{}{}
	return tags, nil
}

// RawAggSeriesQuery is wrapper for rawAggSeriesQuery setting the multi argument to false.
func RawAggSeriesQuery(prefix string, e *expr.State, query, stepDuration, sdur, edur string) (*expr.ResultSet, error) {
	return rawAggSeriesQuery(prefix, e, query, stepDuration, sdur, edur, false)
}

// MRawAggSeriesQuery is wrapper for rawAggSeriesQuery setting the multi argument to true.
func MRawAggSeriesQuery(prefix string, e *expr.State, query, stepDuration, sdur, edur string) (*expr.ResultSet, error) {
	return rawAggSeriesQuery(prefix, e, query, stepDuration, sdur, edur, true)
}

// rawAggSeriesQuery takes a raw promql query that has a top level promql aggregation function
// and returns a seriesSet. If multi is true then the promMultiKey is added to each series in the result
// and multiple prometheus tsdbs are queried.
func rawAggSeriesQuery(prefix string, e *expr.State, query, stepDuration, sdur, edur string, multi bool) (r *expr.ResultSet, err error) {
	r = new(expr.ResultSet)
	parsedPromExpr, err := promql.ParseExpr(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse invalid promql expression: %v", err)
	}
	promAgExprNode, ok := parsedPromExpr.(*promql.AggregateExpr)
	if !ok || promAgExprNode == nil {
		return nil, fmt.Errorf("top level expression is not aggregation operation")
	}
	start, end, err := expr.ParseDurationPair(e, sdur, edur)
	if err != nil {
		return
	}
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return
	}
	step := time.Duration(st)
	tagLen := len(promAgExprNode.Grouping)

	prefixes := strings.Split(prefix, ",")

	// Single prom backend case
	if !multi || (len(prefixes) == 1 && prefixes[0] == "") {
		qRes, err := timeRequest(e, prefix, query, start, end, step)
		if err != nil {
			return nil, err
		}
		err = matrixToResults(prefix, e, qRes, tagLen, false, r)
		return r, err
	}

	// Multibackend case
	wg := sync.WaitGroup{}
	wg.Add(len(prefixes))
	resCh := make(chan struct {
		prefix  string
		promVal promModels.Value
	}, len(prefixes))
	errCh := make(chan error, len(prefixes))

	for _, prefix := range prefixes {
		go func(prefix string) {
			defer wg.Done()
			res, err := timeRequest(e, prefix, query, start, end, step)
			resCh <- struct {
				prefix  string
				promVal promModels.Value
			}{prefix, res}
			errCh <- err
		}(prefix)
	}

	wg.Wait()
	close(resCh)
	close(errCh)
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

	for promRes := range resCh {
		err = matrixToResults(promRes.prefix, e, promRes.promVal, tagLen, true, r)
		if err != nil {
			return
		}
	}

	return
}

// Query is a wrapper for buildAndQuery so there is a function signature that doesn't require the rate argument in the expr language.
// It also sets buildAndQuery's addPrefixTag argument to false since this only queries one backend.
func Query(prefix string, e *expr.State, metric, groupBy, filter, agType, stepDuration, sdur, edur string) (r *expr.ResultSet, err error) {
	return buildAndQuery(prefix, e, metric, groupBy, filter, agType, "", stepDuration, sdur, edur, false)
}

// Rate is a wrapper for buildAndQuery like PromQuery except that it has a rateDuration argument for the step of the rate calculation.
// This enables rate calculation for counters.
func Rate(prefix string, e *expr.State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *expr.ResultSet, err error) {
	return buildAndQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, false)
}

// MQuery is a wrapper from mQuery.
func MQuery(prefix string, e *expr.State, metric, groupBy, filter, agType, stepDuration, sdur, edur string) (r *expr.ResultSet, err error) {
	return mQuery(prefix, e, metric, groupBy, filter, agType, "", stepDuration, sdur, edur)
}

// MRate is a wrapper from mQuery. It has a stepDuration argument for rate calculation.
func MRate(prefix string, e *expr.State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *expr.ResultSet, err error) {
	return mQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur)
}

// mQuery makes call to multiple prometheus TSDBs and combines the results into a single series set.
// It adds the multiKey tag key with the value of prefix label to the results. Queries are executed in parallel.
func mQuery(prefix string, e *expr.State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string) (r *expr.ResultSet, err error) {
	r = new(expr.ResultSet)
	prefixes := strings.Split(prefix, ",")
	if len(prefixes) == 1 && prefixes[0] == "" {
		return buildAndQuery("default", e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, true)
	}

	wg := sync.WaitGroup{}
	wg.Add(len(prefixes))
	resCh := make(chan *expr.ResultSet, len(prefixes))
	errCh := make(chan error, len(prefixes))

	for _, prefix := range prefixes {
		go func(prefix string) {
			defer wg.Done()
			res, err := buildAndQuery(prefix, e, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur, true)
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
	resultCollection := []*expr.ResultSet{}
	for res := range resCh {
		resultCollection = append(resultCollection, res)
	}
	if len(resultCollection) == 1 { // no need to merge if there is only one item
		return resultCollection[0], nil
	}
	// Merge the query results into a single seriesSet
	r, err = expr.Merge(e, resultCollection...)
	return
}

// buildAndQuery uses the information passed to it to generate an PromQL query using the queryTemplate.
// It then calls timeRequest to execute the query and process that results in to a Bosun Results object.
func buildAndQuery(prefix string, e *expr.State, metric, groupBy, filter, agType, rateDuration, stepDuration, sdur, edur string, addPrefixTag bool) (r *expr.ResultSet, err error) {
	r = new(expr.ResultSet)
	start, end, err := expr.ParseDurationPair(e, sdur, edur)
	if err != nil {
		return
	}
	st, err := opentsdb.ParseDuration(stepDuration)
	if err != nil {
		return
	}
	step := time.Duration(st)
	qd := queryTemplateData{
		Metric:       metric,
		AgFunc:       agType,
		Tags:         groupBy,
		Filter:       filter,
		RateDuration: rateDuration,
	}
	query, err := qd.RenderString()
	qRes, err := timeRequest(e, prefix, query, start, end, step)
	if err != nil {
		return
	}
	groupByTagSet := make(opentsdb.TagSet)
	for _, v := range strings.Split(groupBy, ",") {
		if v != "" {
			groupByTagSet[v] = ""
		}
	}
	err = matrixToResults(prefix, e, qRes, len(groupByTagSet), addPrefixTag, r)
	return r, err
}

// queryTemplate is a template for PromQL time series queries. It supports
// filtering and aggregation.
var queryTemplate = template.Must(template.New("queryTemplate").Parse(`
{{ .AgFunc }}(
{{- if ne .RateDuration "" }}rate({{ end }} {{ .Metric -}}
{{- if ne .Filter "" }} {{ .Filter | printf "{%v} " -}} {{- end -}}
{{- if ne .RateDuration "" -}} {{ .RateDuration | printf " [%v] )"  }} {{- end -}}
) by ( {{ .Tags }} )`))

// queryTemplateData is the struct the contains the fields to render the promQueryTemplate.
type queryTemplateData struct {
	Metric       string
	AgFunc       string
	Tags         string
	Filter       string
	RateDuration string
}

// RenderString creates a query string using queryTemplate.
func (pq queryTemplateData) RenderString() (string, error) {
	buf := new(bytes.Buffer)
	err := queryTemplate.Execute(buf, pq)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// timeRequest takes a PromQL query string with the given time frame and step duration. The result
// type of the PromQL query must be a Prometheus Matrix.
func timeRequest(e *expr.State, prefix, query string, start, end time.Time, step time.Duration) (s promModels.Value, err error) {
	client, found := e.Prometheus[prefix]
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
		expr.CollectCacheHit(e.Cache, "prom_ts", hit)
		var ok bool
		if s, ok = val.(promModels.Matrix); !ok {
			err = fmt.Errorf("prom: did not get valid result from prometheus, %v", err)
		}
	})
	return
}

// matrixToResults takes the Value result of a prometheus response and
// updates the Results property of the passed expr.Results object
func matrixToResults(prefix string, e *expr.State, res promModels.Value, expectedTagLen int, addPrefix bool, r *expr.ResultSet) (err error) {
	matrix, ok := res.(promModels.Matrix)
	if !ok {
		return fmt.Errorf("result not of type matrix")
	}
	for _, row := range matrix {
		tags := make(opentsdb.TagSet)
		for tagK, tagV := range row.Metric {
			tags[string(tagK)] = string(tagV)
		}
		// Remove results with less tag keys than those requests
		if len(tags) < expectedTagLen {
			continue
		}
		if addPrefix {
			tags[multiKey] = prefix
		}
		if e.Squelched(tags) {
			continue
		}
		values := make(expr.Series, len(row.Values))
		for _, v := range row.Values {
			values[v.Timestamp.Time()] = float64(v.Value)
		}
		r.Results = append(r.Results, &expr.Result{
			Value: values,
			Group: tags,
		})
	}
	return
}

// MetricList returns a list of available metrics for the prometheus backend
// by using querying the Prometheus Label Values API for "__name__"
func MetricList(prefix string, e *expr.State) (r *expr.ResultSet, err error) {
	r = new(expr.ResultSet)
	client, found := e.Prometheus[prefix]
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
	expr.CollectCacheHit(e.Cache, "prom_metrics", hit)
	if err != nil {
		return nil, err
	}
	metrics := val.(promModels.LabelValues)
	r.Results = append(r.Results, &expr.Result{Value: expr.Info{metrics}})
	return
}

// TagInfo does a range query for the given metric and returns info about the
// tags and labels for the metric based on the data from the queried timeframe
func TagInfo(prefix string, e *expr.State, metric, sdur, edur string) (r *expr.ResultSet, err error) {
	r = new(expr.ResultSet)
	client, found := e.Prometheus[prefix]
	if !found {
		return r, fmt.Errorf(`prometheus client with name "%v" not defined`, prefix)
	}
	start, end, err := expr.ParseDurationPair(e, sdur, edur)
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
	expr.CollectCacheHit(e.Cache, "prom_metrics", hit)
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
	r.Results = append(r.Results, &expr.Result{
		Value: expr.Info{tagInfo},
	})
	return
}
