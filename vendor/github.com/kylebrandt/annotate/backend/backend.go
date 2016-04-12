package backend

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/kylebrandt/annotate"
	elastic "gopkg.in/olivere/elastic.v3"
)

type Backend interface {
	InsertAnnotation(a *annotate.Annotation) error
	GetAnnotation(id string) (*annotate.Annotation, error)
	GetAnnotations(start, end *time.Time, source, host, creationUser, owner, category, message string) (annotate.Annotations, error)
	DeleteAnnotation(id string) error
	GetFieldValues(field string) ([]string, error)
	InitBackend() error
}

const docType = "annotation"

type Elastic struct {
	*elastic.Client
	index      string
	maxResults int
}

func NewElastic(urls []string, index string) (*Elastic, error) {
	e, err := elastic.NewClient(elastic.SetURL(urls...))
	return &Elastic{e, index, 200}, err
}

func (e *Elastic) InsertAnnotation(a *annotate.Annotation) error {
	_, err := e.Index().Index(e.index).BodyJson(a).Id(a.Id).Type(docType).Do()
	return err
}

func (e *Elastic) GetAnnotation(id string) (*annotate.Annotation, error) {
	a := annotate.Annotation{}
	if id == "" {
		return &a, fmt.Errorf("must provide id")
	}
	res, err := e.Get().Index(e.index).Type(docType).Id(id).Do()
	if err != nil {
		return &a, err
	}
	if err := json.Unmarshal(*res.Source, &a); err != nil {
		return &a, err
	}
	return &a, nil
}

func (e *Elastic) DeleteAnnotation(id string) error {
	_, err := e.Delete().Index(e.index).Type(docType).Id(id).Do()
	if err != nil {
		return err
	}
	return nil
	//TODO? Check res.Found?
}

func (e *Elastic) GetAnnotations(start, end *time.Time, source, host, creationUser, owner, category, message string) (annotate.Annotations, error) {
	annotations := annotate.Annotations{}
	filters := []elastic.Query{}
	if start != nil && end != nil {
		startQ := elastic.NewRangeQuery(annotate.EndDate).Gte(start)
		endQ := elastic.NewRangeQuery(annotate.StartDate).Lte(end)
		filters = append(filters, elastic.NewBoolQuery().Must(startQ, endQ))
	}
	if source != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.Source, source))
	}
	if host != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.Host, host))
	}
	if creationUser != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.CreationUser, creationUser))
	}
	if owner != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.Owner, owner))
	}
	if category != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.Category, category))
	}
	if message != "" {
		filters = append(filters, elastic.NewTermQuery(annotate.Message, message))
	}
	res, err := e.Search(e.index).Query(elastic.NewBoolQuery().Must(filters...)).Size(e.maxResults).Do()
	if err != nil {
		return annotations, err
	}
	var aType annotate.Annotation
	for _, item := range res.Each(reflect.TypeOf(aType)) {
		a := item.(annotate.Annotation)
		annotations = append(annotations, a)
	}
	return annotations, nil
}

func (e *Elastic) GetFieldValues(field string) ([]string, error) {
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

func (e *Elastic) InitBackend() error {
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
	mapping := make(map[string]interface{})
	mapping["properties"] = p
	q := e.PutMapping().Index(e.index).Type(docType).BodyJson(mapping)
	res, err := q.Do()
	if (res != nil && !res.Acknowledged) || err != nil {
		return fmt.Errorf("failed to create elastic mapping (ack: %v): %v", res != nil && res.Acknowledged, err)
	}
	return err
}
