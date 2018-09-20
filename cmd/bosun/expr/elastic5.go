// +build esv5

package expr

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	elastic "gopkg.in/olivere/elastic.v5"

	"sync"

	"bosun.org/opentsdb"
	"github.com/jinzhu/now"
)

type ESQuery struct {
	Query elastic.Query
}

// Map of prefixes to corresponding clients
var esClients struct {
	sync.Mutex
	m map[string]*elastic.Client
}

func init() {
	esClients.m = make(map[string]*elastic.Client)
}

func ESAll(e *State) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewMatchAllQuery(),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESAnd(e *State, esqueries ...ESQuery) (*Results, error) {
	var r Results
	queries := make([]elastic.Query, len(esqueries))
	for i, q := range esqueries {
		queries[i] = q.Query
	}
	q := ESQuery{
		Query: elastic.NewBoolQuery().Must(queries...),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESNot(e *State, query ESQuery) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewBoolQuery().MustNot(query.Query),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESOr(e *State, esqueries ...ESQuery) (*Results, error) {
	var r Results
	queries := make([]elastic.Query, len(esqueries))
	for i, q := range esqueries {
		queries[i] = q.Query
	}
	q := ESQuery{
		Query: elastic.NewBoolQuery().Should(queries...).MinimumNumberShouldMatch(1),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESRegexp(e *State, key string, regex string) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRegexpQuery(key, regex),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESQueryString(e *State, key string, query string) (*Results, error) {
	var r Results
	qs := elastic.NewQueryStringQuery(query)
	if key != "" {
		qs.Field(key)
	}
	q := ESQuery{Query: qs}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESExists(e *State, field string) (*Results, error) {
	var r Results
	qs := elastic.NewExistsQuery(field)
	q := ESQuery{Query: qs}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGT(e *State, key string, gt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Gt(gt),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGTE(e *State, key string, gte float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Gte(gte),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLT(e *State, key string, lt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Lt(lt),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLTE(e *State, key string, lte float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Lte(lte),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

// ElasticHosts is an array of Logstash hosts and exists as a type for something to attach
// methods to.  The elasticsearch library will use the listed to hosts to discover all
// of the hosts in the config
// type ElasticHosts []string
type ElasticHosts struct {
	Hosts map[string]ElasticConfig
}

type ElasticConfig struct {
	Hosts             []string
	SimpleClient      bool
	ClientOptionFuncs []elastic.ClientOptionFunc
}

// InitClient sets up the elastic client. If the client has already been
// initialized it is a noop
func (e ElasticHosts) InitClient(prefix string) error {
	if _, ok := e.Hosts[prefix]; !ok {
		prefixes := make([]string, len(e.Hosts))
		i := 0
		for k := range e.Hosts {
			prefixes[i] = k
			i++
		}
		return fmt.Errorf("prefix %v not defined, available prefixes are: %v", prefix, prefixes)
	}
	if c := esClients.m[prefix]; c != nil {
		// client already initialized
		return nil
	}
	esClients.Lock()
	var err error
	if e.Hosts[prefix].SimpleClient {
		// simple client enabled
		esClients.m[prefix], err = elastic.NewSimpleClient(elastic.SetURL(e.Hosts[prefix].Hosts...), elastic.SetMaxRetries(10))
	} else if len(e.Hosts[prefix].Hosts) == 0 {
		// client option enabled
		esClients.m[prefix], err = elastic.NewClient(e.Hosts[prefix].ClientOptionFuncs...)
	} else {
		// default behavior
		esClients.m[prefix], err = elastic.NewClient(elastic.SetURL(e.Hosts[prefix].Hosts...), elastic.SetMaxRetries(10))
	}
	esClients.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// getService returns an elasticsearch service based on the global client
func (e *ElasticHosts) getService(prefix string) (*elastic.SearchService, error) {
	err := e.InitClient(prefix)
	if err != nil {
		return nil, err
	}
	return esClients.m[prefix].Search(), nil
}

// Query takes a Logstash request, applies it a search service, and then queries
// elasticsearch.
func (e ElasticHosts) Query(r *ElasticRequest) (*elastic.SearchResult, error) {
	s, err := e.getService(r.HostKey)
	if err != nil {
		return nil, err
	}

	s.Index(r.Indices...)

	// With IgnoreUnavailable there can be gaps in the indices (i.e. missing days) and we will not error
	// If no indices match than there will be no successful shards and and error is returned in that case
	s.IgnoreUnavailable(true)
	res, err := s.SearchSource(r.Source).Do(context.Background())
	if err != nil {
		return nil, err
	}
	if res.Shards == nil {
		return nil, fmt.Errorf("no shard info in reply, should not be here please file issue")
	}
	if res.Shards.Successful == 0 {
		return nil, fmt.Errorf("no successful shards in result, perhaps the index does exist, total shards: %v, failed shards: %v", res.Shards.Total, res.Shards.Failed)
	}
	return res, nil
}

// ElasticRequest is a container for the information needed to query elasticsearch or a date
// histogram.
type ElasticRequest struct {
	Indices []string
	HostKey string
	Start   *time.Time
	End     *time.Time
	Source  *elastic.SearchSource // This the object that we build queries in
}

// CacheKey returns the text of the elastic query. That text is the indentifer for
// the query in the cache. It is a combination of the host key, indices queries and the json query content
func (r *ElasticRequest) CacheKey() (string, error) {
	s, err := r.Source.Source()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to generate json representation of search source for cache key: %s", s)
	}

	return fmt.Sprintf("%s:%v\n%s", r.HostKey, r.Indices, b), nil
}

// timeESRequest execute the elasticsearch query (which may set or hit cache) and returns
// the search results.
func timeESRequest(e *State, req *ElasticRequest) (resp *elastic.SearchResult, err error) {
	var source interface{}
	source, err = req.Source.Source()
	if err != nil {
		return resp, fmt.Errorf("failed to get source of request while timing elastic request: %s", err)
	}
	b, err := json.MarshalIndent(source, "", "  ")
	if err != nil {
		return resp, err
	}
	key, err := req.CacheKey()
	if err != nil {
		return nil, err
	}
	e.Timer.StepCustomTiming("elastic", "query", fmt.Sprintf("%s:%v\n%s", req.HostKey, req.Indices, b), func() {
		getFn := func() (interface{}, error) {
			return e.ElasticHosts.Query(req)
		}
		var val interface{}
		var hit bool
		val, err, hit = e.Cache.Get(key, getFn)
		collectCacheHit(e.Cache, "elastic", hit)
		resp = val.(*elastic.SearchResult)
	})
	return
}

func ESIndicies(e *State, timeField string, literalIndices ...string) *Results {
	var r Results
	indexer := ESIndexer{}
	// Don't check for existing indexes in this case, just pass through and let elastic return
	// an error at query time if the index does not exist
	indexer.Generate = func(start, end *time.Time) []string {
		return literalIndices
	}
	indexer.TimeField = timeField
	r.Results = append(r.Results, &Result{Value: indexer})
	return &r
}

func ESLS(e *State, indexRoot string) (*Results, error) {
	return ESDaily(e, "@timestamp", indexRoot+"-", "2006.01.02")
}

func ESDaily(e *State, timeField, indexRoot, layout string) (*Results, error) {
	var r Results
	indexer := ESIndexer{}
	indexer.TimeField = timeField
	indexer.Generate = func(start, end *time.Time) []string {
		var indices []string
		truncStart := now.New(*start).BeginningOfDay()
		truncEnd := now.New(*end).BeginningOfDay()
		for d := truncStart; !d.After(truncEnd); d = d.AddDate(0, 0, 1) {
			indices = append(indices, fmt.Sprintf("%v%v", indexRoot, d.Format(layout)))
		}
		return indices
	}
	r.Results = append(r.Results, &Result{Value: indexer})
	return &r, nil
}

func ESMonthly(e *State, timeField, indexRoot, layout string) (*Results, error) {
	var r Results
	indexer := ESIndexer{}
	indexer.TimeField = timeField
	indexer.Generate = func(start, end *time.Time) []string {
		var indices []string
		truncStart := now.New(*start).BeginningOfMonth()
		truncEnd := now.New(*end).BeginningOfMonth()
		for d := truncStart; !d.After(truncEnd); d = d.AddDate(0, 1, 0) {
			indices = append(indices, fmt.Sprintf("%v%v", indexRoot, d.Format(layout)))
		}
		return indices
	}
	r.Results = append(r.Results, &Result{Value: indexer})
	return &r, nil
}

func ESCount(prefix string, e *State, indexer ESIndexer, keystring string, filter ESQuery, interval, sduration, eduration string) (r *Results, err error) {
	return ESDateHistogram(prefix, e, indexer, keystring, filter.Query, interval, sduration, eduration, "", "", 0)
}

// ESStat returns a bucketed statistical reduction for the specified field.
func ESStat(prefix string, e *State, indexer ESIndexer, keystring string, filter ESQuery, field, rstat, interval, sduration, eduration string) (r *Results, err error) {
	return ESDateHistogram(prefix, e, indexer, keystring, filter.Query, interval, sduration, eduration, field, rstat, 0)
}

// 2016-09-22T22:26:14.679270711Z
const elasticRFC3339 = "date_optional_time"

func ESDateHistogram(prefix string, e *State, indexer ESIndexer, keystring string, filter elastic.Query, interval, sduration, eduration, stat_field, rstat string, size int) (r *Results, err error) {
	r = new(Results)
	req, err := ESBaseQuery(e.now, indexer, filter, sduration, eduration, size, prefix)
	if err != nil {
		return nil, err
	}
	// Extended bounds and min doc count are required to get values back when the bucket value is 0
	ts := elastic.NewDateHistogramAggregation().Field(indexer.TimeField).Interval(strings.Replace(interval, "M", "n", -1)).MinDocCount(0).ExtendedBoundsMin(req.Start).ExtendedBoundsMax(req.End).Format(elasticRFC3339)
	if stat_field != "" {
		ts = ts.SubAggregation("stats", elastic.NewExtendedStatsAggregation().Field(stat_field))
		switch rstat {
		case "avg", "min", "max", "sum", "sum_of_squares", "variance", "std_deviation":
		default:
			return r, fmt.Errorf("stat function %v not a valid option", rstat)
		}
	}
	if keystring == "" {
		req.Source = req.Source.Aggregation("ts", ts)
		result, err := timeESRequest(e, req)
		if err != nil {
			return nil, err
		}
		ts, found := result.Aggregations.DateHistogram("ts")
		if !found {
			return nil, fmt.Errorf("expected time series not found in elastic reply")
		}
		series := make(Series)
		for _, v := range ts.Buckets {
			val := processESBucketItem(v, rstat)
			if val != nil {
				series[time.Unix(int64(v.Key)/1000, 0).UTC()] = *val
			}
		}
		if len(series) == 0 {
			return r, nil
		}
		r.Results = append(r.Results, &Result{
			Value: series,
			Group: make(opentsdb.TagSet),
		})
		return r, nil
	}
	keys := strings.Split(keystring, ",")
	aggregation := elastic.NewTermsAggregation().Field(keys[len(keys)-1])
	aggregation = aggregation.SubAggregation("ts", ts)
	for i := len(keys) - 2; i > -1; i-- {
		aggregation = elastic.NewTermsAggregation().Field(keys[i]).SubAggregation("g_"+keys[i+1], aggregation)
	}
	req.Source = req.Source.Aggregation("g_"+keys[0], aggregation)
	result, err := timeESRequest(e, req)
	if err != nil {
		return nil, err
	}
	top, ok := result.Aggregations.Terms("g_" + keys[0])
	if !ok {
		return nil, fmt.Errorf("top key g_%v not found in result", keys[0])
	}
	var desc func(*elastic.AggregationBucketKeyItem, opentsdb.TagSet, []string) error
	desc = func(b *elastic.AggregationBucketKeyItem, tags opentsdb.TagSet, keys []string) error {
		if ts, found := b.DateHistogram("ts"); found {
			if e.Squelched(tags) {
				return nil
			}
			series := make(Series)
			for _, v := range ts.Buckets {
				val := processESBucketItem(v, rstat)
				if val != nil {
					series[time.Unix(int64(v.Key)/1000, 0).UTC()] = *val
				}
			}
			if len(series) == 0 {
				return nil
			}
			r.Results = append(r.Results, &Result{
				Value: series,
				Group: tags.Copy(),
			})
			return nil
		}
		if len(keys) < 1 {
			return nil
		}
		n, _ := b.Aggregations.Terms("g_" + keys[0])
		for _, item := range n.Buckets {
			key := fmt.Sprint(item.Key)
			tags[keys[0]] = key
			if err := desc(item, tags.Copy(), keys[1:]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, b := range top.Buckets {
		tags := make(opentsdb.TagSet)
		key := fmt.Sprint(b.Key)
		tags[keys[0]] = key
		if err := desc(b, tags, keys[1:]); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// ESBaseQuery builds the base query that both ESCount and ESStat share
func ESBaseQuery(now time.Time, indexer ESIndexer, filter elastic.Query, sduration, eduration string, size int, prefix string) (*ElasticRequest, error) {
	start, err := opentsdb.ParseDuration(sduration)
	if err != nil {
		return nil, err
	}
	var end opentsdb.Duration
	if eduration != "" {
		end, err = opentsdb.ParseDuration(eduration)
		if err != nil {
			return nil, err
		}
	}
	st := now.Add(time.Duration(-start))
	en := now.Add(time.Duration(-end))
	indices := indexer.Generate(&st, &en)
	r := ElasticRequest{
		Indices: indices,
		HostKey: prefix,
		Start:   &st,
		End:     &en,
		Source:  elastic.NewSearchSource().Size(size),
	}
	var q elastic.Query
	q = elastic.NewRangeQuery(indexer.TimeField).Gte(st).Lte(en).Format(elasticRFC3339)
	r.Source = r.Source.Query(elastic.NewBoolQuery().Must(q, filter))
	return &r, nil
}

func ScopeES(ts opentsdb.TagSet, q elastic.Query) elastic.Query {
	var filters []elastic.Query
	for tagKey, tagValue := range ts {
		filters = append(filters, elastic.NewTermQuery(tagKey, tagValue))
	}
	filters = append(filters, q)
	b := elastic.NewBoolQuery().Must(filters...)
	return b
}

func processESBucketItem(b *elastic.AggregationBucketHistogramItem, rstat string) *float64 {
	if stats, found := b.ExtendedStats("stats"); found {
		var val *float64
		switch rstat {
		case "avg":
			val = stats.Avg
		case "min":
			val = stats.Min
		case "max":
			val = stats.Max
		case "sum":
			val = stats.Sum
		case "sum_of_squares":
			val = stats.SumOfSquares
		case "variance":
			val = stats.Variance
		case "std_deviation":
			val = stats.StdDeviation
		}
		return val
	}
	v := float64(b.DocCount)
	return &v
}
