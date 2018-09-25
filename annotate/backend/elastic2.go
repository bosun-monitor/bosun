package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"bosun.org/annotate"
	elastic "gopkg.in/olivere/elastic.v3"
)

type Elastic2 struct {
	*elastic.Client
	index             string
	urls              []string
	simpleClient      bool
	clientOptionFuncs []elastic.ClientOptionFunc
	maxResults        int
	initialized       bool
}

func NewElastic2(urls []string, simpleclient bool, index string, clientoptions []elastic.ClientOptionFunc) *Elastic2 {
	return &Elastic2{&elastic.Client{}, index, urls, simpleclient, clientoptions, 200, false}
}

func (e *Elastic2) GetAnnotations(start, end *time.Time, fieldFilters ...FieldFilter) (annotate.Annotations, error) {
	if !e.initialized {
		return nil, unInitErr
	}
	annotations := annotate.Annotations{}
	filters := []elastic.Query{}
	if start != nil && end != nil {
		startQ := elastic.NewRangeQuery(annotate.EndDate).Gte(start)
		endQ := elastic.NewRangeQuery(annotate.StartDate).Lte(end)
		filters = append(filters, elastic.NewBoolQuery().Must(startQ, endQ))
	}
	for _, filter := range fieldFilters {
		switch filter.Field {
		case annotate.Source, annotate.Host, annotate.CreationUser, annotate.Owner, annotate.Category:
		default:
			return annotations, fmt.Errorf("%v is not a field that can be filtered on", filter.Field)
		}
		var q elastic.Query
		switch filter.Verb {
		case Is, "":
			q = elastic.NewTermQuery(filter.Field, filter.Value)
		case Empty:
			// Can't detect empty on a analyzed field
			if filter.Field == annotate.Message {
				return annotations, fmt.Errorf("message field does not support empty searches")
			}
			q = elastic.NewTermQuery(filter.Field, "")
		default:
			return annotations, fmt.Errorf("%v is not a valid query verb", filter.Verb)
		}
		if filter.Not {
			q = elastic.NewBoolQuery().MustNot(q)
		}
		filters = append(filters, q)
	}

	var aType annotate.Annotation
	scroll := e.Scroll(e.index).Query(elastic.NewBoolQuery().Must(filters...)).Size(e.maxResults).Pretty(true)
	for {
		res, err := scroll.Do()
		if err == io.EOF {
			break
		}
		if err != nil {
			return annotations, err
		}
		for _, item := range res.Each(reflect.TypeOf(aType)) {
			a := item.(annotate.Annotation)
			annotations = append(annotations, a)
		}
	}
	return annotations, nil
}

func (e *Elastic2) GetFieldValues(field string) ([]string, error) {
	if !e.initialized {
		return nil, unInitErr
	}
	terms := []string{}
	switch field {
	case annotate.Source, annotate.Host, annotate.CreationUser, annotate.Owner, annotate.Category:
		//continue
	default:
		return terms, fmt.Errorf("invalid field %v", field)
	}
	termsAgg := elastic.NewTermsAggregation().Field(field)
	res, err := e.Search(e.index).Aggregation(field, termsAgg).Size(e.maxResults).Do()
	if err != nil {
		return terms, err
	}
	b, found := res.Aggregations.Terms(field)
	if !found {
		return terms, fmt.Errorf("expected aggregation %v not found in result", field)
	}
	for _, bucket := range b.Buckets {
		if v, ok := bucket.Key.(string); ok {
			terms = append(terms, v)
		}
	}
	return terms, nil
}

func (e *Elastic2) InitBackend() error {
	var err error
	var ec *elastic.Client

	if e.simpleClient {
		ec, err = elastic.NewSimpleClient(elastic.SetURL(e.urls...))
	} else if len(e.urls) == 0 {
		ec, err = elastic.NewClient(e.clientOptionFuncs...)
	} else {
		ec, err = elastic.NewClient(elastic.SetURL(e.urls...))
	}
	if err != nil {
		return err
	}
	e.Client = ec
	exists, err := e.IndexExists(e.index).Do()
	if err != nil {
		return err
	}
	if !exists {
		res, err := e.CreateIndex(e.index).Do()
		if (res != nil && !res.Acknowledged) || err != nil {
			return fmt.Errorf("failed to create elastic mapping (ack: %v): %v", res != nil && res.Acknowledged, err)
		}
	}
	stringNA := map[string]string{
		"type":  "string",
		"index": "not_analyzed",
	}
	stringA := map[string]string{
		"type": "string",
	}
	date := map[string]string{
		"type": "date",
	}
	p := make(map[string]interface{})
	p[annotate.Message] = stringA
	p[annotate.StartDate] = date
	p[annotate.EndDate] = date
	p[annotate.Source] = stringNA
	p[annotate.Host] = stringNA
	p[annotate.CreationUser] = stringNA
	p[annotate.Owner] = stringNA
	p[annotate.Category] = stringNA
	p[annotate.Url] = stringNA
	mapping := make(map[string]interface{})
	mapping["properties"] = p
	q := e.PutMapping().Index(e.index).Type(docType).BodyJson(mapping)
	res, err := q.Do()
	if (res != nil && !res.Acknowledged) || err != nil {
		return fmt.Errorf("failed to create elastic mapping (ack: %v): %v", res != nil && res.Acknowledged, err)
	}
	e.initialized = true
	return nil
}

func (e *Elastic2) InsertAnnotation(a *annotate.Annotation) error {
	if !e.initialized {
		return unInitErr
	}
	_, err := e.Index().Index(e.index).BodyJson(a).Id(a.Id).Type(docType).Do()
	return err
}

func (e *Elastic2) GetAnnotation(id string) (*annotate.Annotation, bool, error) {
	if !e.initialized {
		return nil, false, unInitErr
	}
	a := annotate.Annotation{}
	if id == "" {
		return &a, false, fmt.Errorf("must provide id")
	}
	res, err := e.Get().Index(e.index).Type(docType).Id(id).Do()
	// Ewwww...
	if err != nil && strings.Contains(err.Error(), "Error 404") {
		return &a, false, nil
	}
	if err != nil {
		return &a, false, err
	}
	if err := json.Unmarshal(*res.Source, &a); err != nil {
		return &a, res.Found, err
	}
	return &a, res.Found, nil
}

func (e *Elastic2) DeleteAnnotation(id string) error {
	if !e.initialized {
		return unInitErr
	}
	_, err := e.Delete().Index(e.index).Type(docType).Id(id).Do()
	if err != nil {
		return err
	}
	return nil
	//TODO? Check res.Found?
}
