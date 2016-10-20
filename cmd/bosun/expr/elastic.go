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
	"github.com/jinzhu/now"
	elastic "gopkg.in/olivere/elastic.v3"
)

// This uses a global client since the elastic client handles connections
var esClient *elastic.Client

func elasticTagQuery(args []parse.Node) (parse.Tags, error) {
	n := args[1].(*parse.StringNode)
	t := make(parse.Tags)
	for _, s := range strings.Split(n.Text, ",") {
		t[s] = struct{}{}
	}
	return t, nil
}

// ElasticFuncs are specific functions that query an elasticsearch instance.
// They are only loaded when the elastic hosts are set in the config file
var Elastic = map[string]parse.Func{
	// Funcs for querying elastic
	"escount": {
		Args:   []models.FuncType{models.TypeESIndexer, models.TypeString, models.TypeESQuery, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   elasticTagQuery,
		F:      ESCount,
	},
	"esstat": {
		Args:   []models.FuncType{models.TypeESIndexer, models.TypeString, models.TypeESQuery, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   elasticTagQuery,
		F:      ESStat,
	},

	// Funcs to create elastic index names (ESIndexer type)
	"esindices": {
		Args:     []models.FuncType{models.TypeString, models.TypeString},
		VArgs:    true,
		VArgsPos: 1,
		Return:   models.TypeESIndexer,
		F:        ESIndicies,
	},
	"esdaily": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESDaily,
	},
	"esmonthly": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESMonthly,
	},
	"esls": {
		Args:   []models.FuncType{models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESLS,
	},

	// Funcs for generate elastic queries (ESQuery Type) to further filter results
	"esall": {
		Args:   []models.FuncType{},
		Return: models.TypeESQuery,
		F:      ESAll,
	},
	"esregexp": {
		Args:   []models.FuncType{models.TypeString, models.TypeString},
		Return: models.TypeESQuery,
		F:      ESRegexp,
	},
	"esquery": {
		Args:   []models.FuncType{models.TypeString, models.TypeString},
		Return: models.TypeESQuery,
		F:      ESQueryString,
	},
	"esexists": {
		Args:   []models.FuncType{models.TypeString},
		Return: models.TypeESQuery,
		F:      ESExists,
	},
	"esand": {
		Args:   []models.FuncType{models.TypeESQuery},
		VArgs:  true,
		Return: models.TypeESQuery,
		F:      ESAnd,
	},
	"esor": {
		Args:   []models.FuncType{models.TypeESQuery},
		VArgs:  true,
		Return: models.TypeESQuery,
		F:      ESOr,
	},
	"esnot": {
		Args:   []models.FuncType{models.TypeESQuery},
		Return: models.TypeESQuery,
		F:      ESNot,
	},
	"esgt": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESGT,
	},
	"esgte": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESGTE,
	},
	"eslt": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESLT,
	},
	"eslte": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESLTE,
	},
}

func ESAll(e *State, T miniprofiler.Timer) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewMatchAllQuery(),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESAnd(e *State, T miniprofiler.Timer, esqueries ...ESQuery) (*Results, error) {
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

func ESNot(e *State, T miniprofiler.Timer, query ESQuery) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewBoolQuery().MustNot(query.Query),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESOr(e *State, T miniprofiler.Timer, esqueries ...ESQuery) (*Results, error) {
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

func ESRegexp(e *State, T miniprofiler.Timer, key string, regex string) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRegexpQuery(key, regex),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESQueryString(e *State, T miniprofiler.Timer, key string, query string) (*Results, error) {
	var r Results
	qs := elastic.NewQueryStringQuery(query)
	if key != "" {
		qs.Field(key)
	}
	q := ESQuery{Query: qs}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESExists(e *State, T miniprofiler.Timer, field string) (*Results, error) {
	var r Results
	qs := elastic.NewExistsQuery(field)
	q := ESQuery{Query: qs}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGT(e *State, T miniprofiler.Timer, key string, gt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Gt(gt),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGTE(e *State, T miniprofiler.Timer, key string, gte float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Gte(gte),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLT(e *State, T miniprofiler.Timer, key string, lt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: elastic.NewRangeQuery(key).Lt(lt),
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLTE(e *State, T miniprofiler.Timer, key string, lte float64) (*Results, error) {
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
type ElasticHosts []string

// InitClient sets up the elastic client. If the client has already been
// initalized it is a noop
func (e ElasticHosts) InitClient() error {
	if esClient == nil {
		var err error
		esClient, err = elastic.NewClient(elastic.SetURL(e...), elastic.SetMaxRetries(10))
		if err != nil {
			return err
		}
	}
	return nil
}

// getService returns an elasticsearch service based on the global client
func (e *ElasticHosts) getService() (*elastic.SearchService, error) {
	err := e.InitClient()
	if err != nil {
		return nil, err
	}
	return esClient.Search(), nil
}

// Query takes a Logstash request, applies it a search service, and then queries
// elasticsearch.
func (e ElasticHosts) Query(r *ElasticRequest) (*elastic.SearchResult, error) {
	s, err := e.getService()
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	s.Index(r.Indices...)
	// With IgnoreUnavailable there can be gaps in the indices (i.e. missing days) and we will not error
	// If no indices match than there will be no successful shards and and error is returned in that case
	s.IgnoreUnavailable(true)
	res, err := s.SearchSource(r.Source).Do()
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
	Start   *time.Time
	End     *time.Time
	Source  *elastic.SearchSource // This the object that we build queries in
}

// CacheKey returns the text of the elastic query. That text is the indentifer for
// the query in the cache. It is a combination of the indices queries and the json query content
func (r *ElasticRequest) CacheKey() (string, error) {
	s, err := r.Source.Source()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to generate json representation of search source for cache key: %s", s)
	}
	return fmt.Sprintf("%v\n%s", r.Indices, b), nil
}

// timeESRequest execute the elasticsearch query (which may set or hit cache) and returns
// the search results.
func timeESRequest(e *State, T miniprofiler.Timer, req *ElasticRequest) (resp *elastic.SearchResult, err error) {
	e.elasticQueries = append(e.elasticQueries, *req.Source)
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
	T.StepCustomTiming("elastic", "query", fmt.Sprintf("%v\n%s", req.Indices, b), func() {
		getFn := func() (interface{}, error) {
			return e.ElasticHosts.Query(req)
		}
		var val interface{}
		val, err = e.Cache.Get(key, getFn)
		resp = val.(*elastic.SearchResult)
	})
	return
}

func ESIndicies(e *State, T miniprofiler.Timer, timeField string, literalIndices ...string) *Results {
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

func ESLS(e *State, T miniprofiler.Timer, indexRoot string) (*Results, error) {
	return ESDaily(e, T, "@timestamp", indexRoot+"-", "2006.01.02")
}

func ESDaily(e *State, T miniprofiler.Timer, timeField, indexRoot, layout string) (*Results, error) {
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

func ESMonthly(e *State, T miniprofiler.Timer, timeField, indexRoot, layout string) (*Results, error) {
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

func ESCount(e *State, T miniprofiler.Timer, indexer ESIndexer, keystring string, filter ESQuery, interval, sduration, eduration string) (r *Results, err error) {
	return ESDateHistogram(e, T, indexer, keystring, filter.Query, interval, sduration, eduration, "", "", 0)
}

// ESStat returns a bucketed statistical reduction for the specified field.
func ESStat(e *State, T miniprofiler.Timer, indexer ESIndexer, keystring string, filter ESQuery, field, rstat, interval, sduration, eduration string) (r *Results, err error) {
	return ESDateHistogram(e, T, indexer, keystring, filter.Query, interval, sduration, eduration, field, rstat, 0)
}

// 2016-09-22T22:26:14.679270711Z
const elasticRFC3339 = "date_optional_time"

func ESDateHistogram(e *State, T miniprofiler.Timer, indexer ESIndexer, keystring string, filter elastic.Query, interval, sduration, eduration, stat_field, rstat string, size int) (r *Results, err error) {
	r = new(Results)
	req, err := ESBaseQuery(e.now, indexer, filter, sduration, eduration, size)
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
		result, err := timeESRequest(e, T, req)
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
				series[time.Unix(v.Key/1000, 0).UTC()] = *val
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
	aggregation := elastic.NewTermsAggregation().Field(keys[len(keys)-1]).Size(0)
	aggregation = aggregation.SubAggregation("ts", ts)
	for i := len(keys) - 2; i > -1; i-- {
		aggregation = elastic.NewTermsAggregation().Field(keys[i]).Size(0).SubAggregation("g_"+keys[i+1], aggregation)
	}
	req.Source = req.Source.Aggregation("g_"+keys[0], aggregation)
	result, err := timeESRequest(e, T, req)
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
					series[time.Unix(v.Key/1000, 0).UTC()] = *val
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
func ESBaseQuery(now time.Time, indexer ESIndexer, filter elastic.Query, sduration, eduration string, size int) (*ElasticRequest, error) {
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
