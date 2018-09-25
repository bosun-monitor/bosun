package expr

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jinzhu/now"

	elastic6 "github.com/olivere/elastic"
	elastic2 "gopkg.in/olivere/elastic.v3"
	elastic5 "gopkg.in/olivere/elastic.v5"
)

const (
	ESV2 ESVersion = "v2"
	ESV5 ESVersion = "v5"
	ESV6 ESVersion = "v6"

	// 2016-09-22T22:26:14.679270711Z
	elasticRFC3339 = "date_optional_time"
)

type ESVersion string

type ESQuery struct {
	Query func(ver ESVersion) interface{}
}

// Map of prefixes to corresponding clients
// TODO: switch to sync.Map
var esClients struct {
	sync.Mutex
	m map[string]interface{}
}

func init() {
	esClients.m = make(map[string]interface{})
}

func ESAll(e *State) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewMatchAllQuery()
			case ESV5:
				return elastic5.NewMatchAllQuery()
			case ESV6:
				return elastic6.NewMatchAllQuery()
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESAnd(e *State, esqueries ...ESQuery) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				queries := make([]elastic2.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic2.Query)
				}
				return elastic2.NewBoolQuery().Must(queries...)
			case ESV5:
				queries := make([]elastic5.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic5.Query)
				}
				return elastic5.NewBoolQuery().Must(queries...)
			case ESV6:
				queries := make([]elastic6.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic6.Query)
				}
				return elastic6.NewBoolQuery().Must(queries...)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESNot(e *State, query ESQuery) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewBoolQuery().MustNot(query.Query(ver).(elastic2.Query))
			case ESV5:
				return elastic5.NewBoolQuery().MustNot(query.Query(ver).(elastic5.Query))
			case ESV6:
				return elastic6.NewBoolQuery().MustNot(query.Query(ver).(elastic6.Query))
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESOr(e *State, esqueries ...ESQuery) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				queries := make([]elastic2.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic2.Query)
				}
				return elastic2.NewBoolQuery().Should(queries...).MinimumNumberShouldMatch(1)
			case ESV5:
				queries := make([]elastic5.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic5.Query)
				}
				return elastic5.NewBoolQuery().Should(queries...).MinimumNumberShouldMatch(1)
			case ESV6:
				queries := make([]elastic6.Query, len(esqueries))
				for i, q := range esqueries {
					queries[i] = q.Query(ver).(elastic6.Query)
				}
				return elastic6.NewBoolQuery().Should(queries...).MinimumNumberShouldMatch(1)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESRegexp(e *State, key string, regex string) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewRegexpQuery(key, regex)
			case ESV5:
				return elastic5.NewRegexpQuery(key, regex)
			case ESV6:
				return elastic6.NewRegexpQuery(key, regex)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESQueryString(e *State, key string, query string) (*Results, error) {
	var r Results
	q := ESQuery{
		// Query: qs
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				qs := elastic2.NewQueryStringQuery(query)
				if key != "" {
					qs.Field(key)
				}
				return qs
			case ESV5:
				qs := elastic5.NewQueryStringQuery(query)
				if key != "" {
					qs.Field(key)
				}
				return qs
			case ESV6:
				qs := elastic6.NewQueryStringQuery(query)
				if key != "" {
					qs.Field(key)
				}
				return qs
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESExists(e *State, field string) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewExistsQuery(field)
			case ESV5:
				return elastic5.NewExistsQuery(field)
			case ESV6:
				return elastic6.NewExistsQuery(field)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGT(e *State, key string, gt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewRangeQuery(key).Gt(gt)
			case ESV5:
				return elastic5.NewRangeQuery(key).Gt(gt)
			case ESV6:
				return elastic6.NewRangeQuery(key).Gt(gt)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESGTE(e *State, key string, gte float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewRangeQuery(key).Gte(gte)
			case ESV5:
				return elastic5.NewRangeQuery(key).Gte(gte)
			case ESV6:
				return elastic6.NewRangeQuery(key).Gte(gte)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLT(e *State, key string, lt float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewRangeQuery(key).Lt(lt)
			case ESV5:
				return elastic5.NewRangeQuery(key).Lt(lt)
			case ESV6:
				return elastic6.NewRangeQuery(key).Lt(lt)
			}
			return nil
		},
	}
	r.Results = append(r.Results, &Result{Value: q})
	return &r, nil
}

func ESLTE(e *State, key string, lte float64) (*Results, error) {
	var r Results
	q := ESQuery{
		Query: func(ver ESVersion) interface{} {
			switch ver {
			case ESV2:
				return elastic2.NewRangeQuery(key).Lte(lte)
			case ESV5:
				return elastic5.NewRangeQuery(key).Lte(lte)
			case ESV6:
				return elastic6.NewRangeQuery(key).Lte(lte)
			}
			return nil
		},
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
	Version           ESVersion
	SimpleClient      bool
	ClientOptionFuncs interface{}
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
	var err error
	if e.Hosts[prefix].SimpleClient {
		// simple client enabled
		err = createVersionedSimpleESClient(prefix, e.Hosts[prefix])
	} else {
		// default behavior
		err = createVersionedESClient(prefix, e.Hosts[prefix])
	}
	if err != nil {
		return err
	}
	return nil
}

func createVersionedSimpleESClient(prefix string, cfg ElasticConfig) error {
	var err error
	switch cfg.Version {
	case ESV2:
		esClients.m[prefix], err = elastic2.NewSimpleClient(elastic2.SetURL(cfg.Hosts...), elastic2.SetMaxRetries(10))
	case ESV5:
		esClients.m[prefix], err = elastic5.NewSimpleClient(elastic5.SetURL(cfg.Hosts...), elastic5.SetMaxRetries(10))
	case ESV6:
		esClients.m[prefix], err = elastic6.NewSimpleClient(elastic6.SetURL(cfg.Hosts...), elastic6.SetMaxRetries(10))
	}
	return err
}

func createVersionedESClient(prefix string, cfg ElasticConfig) error {
	var err error
	switch cfg.Version {
	case ESV2:
		if len(cfg.Hosts) == 0 {
			// client option enabled
			esClients.m[prefix], err = elastic2.NewClient(cfg.ClientOptionFuncs.([]elastic2.ClientOptionFunc)...)
		} else {
			// default behavior
			esClients.m[prefix], err = elastic2.NewClient(elastic2.SetURL(cfg.Hosts...), elastic2.SetMaxRetries(10))
		}
	case ESV5:
		if len(cfg.Hosts) == 0 {
			// client option enabled
			esClients.m[prefix], err = elastic5.NewClient(cfg.ClientOptionFuncs.([]elastic5.ClientOptionFunc)...)
		} else {
			// default behavior
			esClients.m[prefix], err = elastic5.NewClient(elastic5.SetURL(cfg.Hosts...), elastic5.SetMaxRetries(10))
		}
	case ESV6:
		if len(cfg.Hosts) == 0 {
			// client option enabled
			esClients.m[prefix], err = elastic6.NewClient(cfg.ClientOptionFuncs.([]elastic6.ClientOptionFunc)...)
		} else {
			// default behavior
			esClients.m[prefix], err = elastic6.NewClient(elastic6.SetURL(cfg.Hosts...), elastic6.SetMaxRetries(10))
		}
	}
	return err
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
	switch ver := e.ElasticHosts.Hosts[prefix].Version; ver {
	case ESV2:
		return ESDateHistogram2(prefix, e, indexer, keystring, filter.Query(ver).(elastic2.Query), interval, sduration, eduration, "", "", 0)
	case ESV5:
		return ESDateHistogram5(prefix, e, indexer, keystring, filter.Query(ver).(elastic5.Query), interval, sduration, eduration, "", "", 0)
	case ESV6:
		return ESDateHistogram6(prefix, e, indexer, keystring, filter.Query(ver).(elastic6.Query), interval, sduration, eduration, "", "", 0)
	}
	return nil, errors.New("unknown version")
}

// ESStat returns a bucketed statistical reduction for the specified field.
func ESStat(prefix string, e *State, indexer ESIndexer, keystring string, filter ESQuery, field, rstat, interval, sduration, eduration string) (r *Results, err error) {
	switch ver := e.ElasticHosts.Hosts[prefix].Version; ver {
	case ESV2:
		return ESDateHistogram2(prefix, e, indexer, keystring, filter.Query(ver).(elastic2.Query), interval, sduration, eduration, field, rstat, 0)
	case ESV5:
		return ESDateHistogram5(prefix, e, indexer, keystring, filter.Query(ver).(elastic5.Query), interval, sduration, eduration, field, rstat, 0)
	case ESV6:
		return ESDateHistogram6(prefix, e, indexer, keystring, filter.Query(ver).(elastic6.Query), interval, sduration, eduration, field, rstat, 0)
	}
	return nil, errors.New("unknown version")
}
