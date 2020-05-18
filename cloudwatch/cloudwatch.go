// Package cloudwatch defines structures for interacting with Cloudwatch Metrics.
package cloudwatch // import "bosun.org/cloudwatch"

import (
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
	cwi "github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/ryanuber/go-glob"
	"strings"
	"sync"
	"time"
)

var (
	once    sync.Once
	context Context
)

const DefaultConcurrency = 4
const DefaultPageLimit = 10
const DefaultExpansionLimit = 500

var ErrExpansionLimit = errors.New("Hit dimension expansion limit")
var ErrPagingLimit = errors.New("Hit the page limit when retrieving metrics")
var ErrInvalidPeriod = errors.New("Period must be greater than 0")

// Request holds query objects. Currently only absolute times are supported.
type Request struct {
	Start           *time.Time
	End             *time.Time
	Region          string
	Namespace       string
	Metric          string
	Period          int64
	Statistic       string
	DimensionString string
	Dimensions      [][]Dimension
	Profile         string
}
type LookupRequest struct {
	Region     string
	Namespace  string
	Metric     string
	Dimensions [][]Dimension
	Profile    string
}

type Response struct {
	Raw    cw.GetMetricDataOutput
	TagSet map[string]opentsdb.TagSet
}

type Series struct {
	Datapoints []DataPoint
	Label      string
}

type DataPoint struct {
	Aggregator string
	Timestamp  string
	Unit       string
}

type Dimension struct {
	Name  string
	Value string
}

type Wildcards map[string]string

type DimensionSet map[string]bool

func (d Dimension) String() string {
	return fmt.Sprintf("%s:%s", d.Name, d.Value)
}

type DimensionList struct {
	Groups [][]Dimension
}

func (r *Request) CacheKey() string {
	return fmt.Sprintf("cloudwatch-%d-%d-%s-%s-%s-%d-%s-%s-%s",
		r.Start.Unix(),
		r.End.Unix(),
		r.Region,
		r.Namespace,
		r.Metric,
		r.Period,
		r.Statistic,
		r.DimensionString,
		r.Profile,
	)
}

// Context is the interface for querying CloudWatch.
type Context interface {
	Query(*Request) (Response, error)
	LookupDimensions(request *LookupRequest) ([][]Dimension, error)
	GetExpansionLimit() int
	GetPagesLimit() int
	GetConcurrency() int
}

type cloudWatchContext struct {
	profileProvider ProfileProvider
	profiles        map[string]cwi.CloudWatchAPI
	profilesLock    sync.RWMutex
	ExpansionLimit  int
	PagesLimit      int
	Concurrency     int
}

type ProfileProvider interface {
	NewProfile(name, region string) cwi.CloudWatchAPI
}

type profileProvider struct{}

func (p profileProvider) NewProfile(name, region string) cwi.CloudWatchAPI {
	enableVerboseLogging := true
	conf := aws.Config{
		CredentialsChainVerboseErrors: &enableVerboseLogging,
		Region:                        aws.String(region),
	}

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: name,
		Config:  conf,
		// Force enable Shared Config support
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		slog.Error(err.Error())
	}

	return cw.New(sess)
}

// getProfile returns a previously created profile or creates a new one for the given profile name and region
func (c *cloudWatchContext) getProfile(awsProfileName, region string) cwi.CloudWatchAPI {
	var fullProfileName string

	if awsProfileName == "default" {
		fullProfileName = "bosun-default"
	} else {
		fullProfileName = fmt.Sprintf("user-%s", awsProfileName)
	}

	fullProfileName = fmt.Sprintf("%s-%s", fullProfileName, region)

	// We don't want to concurrently modify the c.profiles map
	c.profilesLock.Lock()
	defer c.profilesLock.Unlock()

	if cwAPI, ok := c.profiles[fullProfileName]; ok {
		return cwAPI
	}

	cwAPI := c.profileProvider.NewProfile(awsProfileName, region)
	c.profiles[fullProfileName] = cwAPI

	return cwAPI
}

func (c *cloudWatchContext) GetPagesLimit() int {
	if c.PagesLimit == 0 {
		return DefaultPageLimit
	} else {
		return c.PagesLimit

	}
}

func (c *cloudWatchContext) GetExpansionLimit() int {
	if c.ExpansionLimit == 0 {
		return DefaultExpansionLimit
	} else {
		return c.ExpansionLimit

	}
}

func (c *cloudWatchContext) GetConcurrency() int {
	if c.Concurrency == 0 {
		return DefaultConcurrency
	} else {
		return c.Concurrency

	}
}

func GetContext() Context {
	return GetContextWithProvider(profileProvider{})
}

func GetContextWithProvider(p ProfileProvider) Context {
	once.Do(func() {
		context = &cloudWatchContext{
			profileProvider: p,
			profiles:        make(map[string]cwi.CloudWatchAPI),
		}
	})
	return context
}

func buildQuery(r *Request, id string, dimensions []Dimension) cw.MetricDataQuery {
	awsPeriod := r.Period
	d := make([]*cw.Dimension, 0)

	for _, i := range dimensions {
		n := i.Name
		v := i.Value
		d = append(d, &cw.Dimension{Name: &n, Value: &v})
	}

	metric := cw.Metric{
		Dimensions: d,
		MetricName: &r.Metric,
		Namespace:  &r.Namespace,
	}
	stat := cw.MetricStat{
		Metric: &metric,
		Period: &awsPeriod,
		Stat:   &r.Statistic,
		Unit:   nil,
	}

	returndata := true
	dq := cw.MetricDataQuery{
		Expression: nil,
		Id:         &id,
		Label:      nil,
		MetricStat: &stat,
		Period:     nil,
		ReturnData: &returndata,
	}
	return dq
}

func buildTags(dims []Dimension) opentsdb.TagSet {
	var tags opentsdb.TagSet

	tags = make(opentsdb.TagSet)
	for _, d := range dims {
		tags[d.Name] = d.Value
	}

	return tags
}

// Query performs a CloudWatch request to aws.
func (c cloudWatchContext) Query(r *Request) (Response, error) {
	var response Response
	var dqs []*cw.MetricDataQuery
	var tagSet = make(map[string]opentsdb.TagSet)
	var id string

	api := c.getProfile(r.Profile, r.Region)

	if r.Period <= 0 {
		return response, ErrInvalidPeriod
	}
	// custom metrics can have no dimensions
	if len(r.Dimensions) == 0 {
		id = fmt.Sprintf("q0")
		dq := buildQuery(r, id, nil)
		dqs = append(dqs, &dq)
		tagSet[id] = buildTags(nil)
	} else {
		for i, j := range r.Dimensions {
			id = fmt.Sprintf("q%d", i)
			dq := buildQuery(r, id, j)
			dqs = append(dqs, &dq)
			tagSet[id] = buildTags(j)
		}
	}

	q := &cw.GetMetricDataInput{
		EndTime:           aws.Time(*r.End),
		MaxDatapoints:     nil,
		MetricDataQueries: dqs,
		NextToken:         nil,
		ScanBy:            nil,
		StartTime:         aws.Time(*r.Start),
	}

	resp, err := api.GetMetricData(q)
	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		slog.Error(err.Error())
		return response, err
	}
	response.Raw = *resp
	response.TagSet = tagSet
	return response, nil

}

func match(d *cw.Dimension, wc Wildcards) bool {
	if len(*d.Value) == 0 {
		return false
	}
	return glob.Glob(wc[*d.Name], *d.Value)
}

func filter(metric *cw.Metric, wildcard Wildcards, dimSet DimensionSet) (matches int, dims []Dimension) {
	matches = 0
	dl := make([]Dimension, 0)

	for _, dim := range metric.Dimensions {
		// if the metric contains a dimension that isn't in the list
		// we searched for we should skip it.
		if !dimSet[*dim.Name] {
			return 0, nil
		}

		if wildcard[*dim.Name] != "" {
			if !match(dim, wildcard) {
				return 0, nil
			}
			matches++
		}

		d := Dimension{
			Name:  *dim.Name,
			Value: *dim.Value,
		}
		dl = append(dl, d)
	}
	return matches, dl

}

func filterDimensions(metrics []*cw.Metric, wildcard Wildcards, ds DimensionSet, limit int) ([][]Dimension, error) {
	dimensions := make([][]Dimension, 0)

	for _, m := range metrics {
		if len(m.Dimensions) == 0 {
			continue
		}
		matches, dl := filter(m, wildcard, ds)
		// all wildcard dimensions need to be present for it to count as match
		if matches < len(wildcard) {
			continue
		}
		dimensions = append(dimensions, dl)
		if len(dimensions) >= limit {
			return nil, ErrExpansionLimit
		}
	}
	return dimensions, nil
}

// Query performs a CloudWatch request to aws.
func (c cloudWatchContext) LookupDimensions(lr *LookupRequest) ([][]Dimension, error) {
	api := c.getProfile(lr.Profile, lr.Region)
	var metrics []*cw.Metric
	var literal []*cw.DimensionFilter
	var wildcard = make(Wildcards)
	var dimensionSet = make(DimensionSet)

	for _, i := range lr.Dimensions {
		for _, j := range i {
			dimensionSet[j.Name] = true

			if strings.Contains(j.Value, "*") {
				wildcard[j.Name] = j.Value
			} else {
				name := j.Name
				value := j.Value
				literal = append(literal, &cw.DimensionFilter{
					Name:  &name,
					Value: &value,
				})
			}
		}
	}

	mi := cw.ListMetricsInput{
		Dimensions: literal,
		MetricName: &lr.Metric,
		Namespace:  &lr.Namespace,
		NextToken:  nil,
	}
	pages := 0
	limitHit := false
	err := api.ListMetricsPages(&mi, func(mo *cw.ListMetricsOutput, lastPage bool) bool {
		metrics = append(metrics, mo.Metrics...)
		pages++
		if pages > c.GetPagesLimit() {
			limitHit = true
			return false
		}
		return !lastPage
	})

	if limitHit {
		return nil, ErrPagingLimit
	}
	if err != nil {
		return nil, err
	}

	return filterDimensions(metrics, wildcard, dimensionSet, c.GetExpansionLimit())
}
