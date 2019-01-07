// Package cloudwatch defines structures for interacting with Cloudwatch Metrics.
package cloudwatch // import "bosun.org/cloudwatch"

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"reflect"
	"time"

	"bosun.org/models"
	"bosun.org/slog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
	cwi "github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

var flatAttributes = map[string]bool{
	"AmiLaunchIndex":        true,
	"Architecture":          true,
	"ClientToken":           true,
	"EbsOptimized":          true,
	"EnaSupport":            true,
	"Hypervisor":            true,
	"IamInstanceProfile":    true,
	"ImageId":               true,
	"InstanceId":            true,
	"InstanceLifecycle":     true,
	"InstanceType":          true,
	"KernelId":              true,
	"KeyName":               true,
	"LaunchTime":            true,
	"Platform":              true,
	"PrivateDnsName":        true,
	"PrivateIpAddress":      true,
	"PublicDnsName":         true,
	"PublicIpAddress":       true,
	"RamdiskId":             true,
	"RootDeviceName":        true,
	"RootDeviceType":        true,
	"SourceDestCheck":       true,
	"SpotInstanceRequestId": true,
	"SriovNetSupport":       true,
	"SubnetId":              true,
	"VirtualizationType":    true,
	"VpcId":                 true}

// Request holds query objects. Currently only absolute times are supported.
type Request struct {
	Start      *time.Time
	End        *time.Time
	Region     string
	Namespace  string
	Metric     string
	Period     int64
	Statistic  string
	Dimensions []Dimension
	Profile    string
}

type Response struct {
	Raw cw.GetMetricStatisticsOutput
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

func (d Dimension) String() string {
	return fmt.Sprintf("%s:%s", d.Name, d.Value)
}

type DimensionList struct {
	Groups [][]Dimension
}

func (c DimensionList) Type() models.FuncType { return models.TypeCWDimensionList }
func (c DimensionList) Value() interface{}    { return c }

func (r *Request) CacheKey() string {
	return fmt.Sprintf("cloudwatch-%d-%d-%s-%s-%s-%d-%s-%s-%s",
		r.Start.Unix(),
		r.End.Unix(),
		r.Region,
		r.Namespace,
		r.Metric,
		r.Period,
		r.Statistic,
		r.Dimensions,
		r.Profile,
	)
}

// Perform a query to cloudwatch
func (r *Request) Query(svc cwi.CloudWatchAPI) (Response, error) {

	var response Response
	awsPeriod := r.Period

	dimensions := make([]*cw.Dimension, 0)
	for _, i := range r.Dimensions {
		dimensions = append(dimensions, &cw.Dimension{
			Name:  aws.String(i.Name),
			Value: aws.String(i.Value),
		})
	}

	search := &cw.GetMetricStatisticsInput{
		StartTime:  aws.Time(*r.Start),
		EndTime:    aws.Time(*r.End),
		MetricName: aws.String(r.Metric),
		Period:     &awsPeriod,
		Statistics: []*string{aws.String(r.Statistic)},
		Namespace:  aws.String(r.Namespace),
		Dimensions: dimensions,
	}
	resp, err := svc.GetMetricStatistics(search)
	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		slog.Error(err.Error())
		return response, err
	}
	response.Raw = *resp
	return response, nil
}

// Context is the interface for querying cloudwatch.
type Context interface {
	Query(*Request) (Response, error)
	Describe(region, prefix, attribute string, filter []*ec2.Filter) (*DimensionList, error)
	GetConcurrency() int
}

type Config struct {
	CWProfiles  map[string]cwi.CloudWatchAPI
	EC2Profiles map[string]ec2iface.EC2API
	Concurrency int
}

func NewConfig(concurrency int) *Config {
	c := new(Config)
	c.CWProfiles = make(map[string]cwi.CloudWatchAPI)
	c.EC2Profiles = make(map[string]ec2iface.EC2API)
	c.Concurrency = concurrency
	return c
}

func (c Config) GetConcurrency() int {
	return c.Concurrency
}

// Query performs a cloudwatch request to aws.
func (c Config) Query(r *Request) (Response, error) {
	var profile string
	var conf aws.Config
	if r.Profile == "default" {
		profile = "bosun-default"
	} else {
		profile = "user-" + r.Profile
	}
	// if the session hasn't already been initialised for this profile create a new one
	if c.CWProfiles[profile] == nil {
		conf.Credentials = credentials.NewSharedCredentials("", r.Profile)
		conf.Region = aws.String(r.Region)
		c.CWProfiles[profile] = cw.New(session.New(&conf))
	}

	return r.Query(c.CWProfiles[profile])
}

// is the ec2 instance attribute a single level value suitable for use in a query
func isFlatAttribute(attribute string) bool {
	_, ok := flatAttributes[attribute]
	return ok
}

// call ec2_describe and generate a dimension list
func (c Config) Describe(region, profile, attribute string, filters []*ec2.Filter) (*DimensionList, error) {
	var conf aws.Config
	var p string
	if profile == "default" {
		p = "bosun-default"
	} else {
		p = "user-" + profile
	}
	// if the session hasn't already been initialised for this profile create a new one
	if c.EC2Profiles[p] == nil {
		conf.Credentials = credentials.NewSharedCredentials("", profile)
		conf.Region = aws.String(region)
		c.EC2Profiles[p] = ec2.New(session.New(&conf))
	}
	svc := c.EC2Profiles[p]

	list := DimensionList{}

	if !isFlatAttribute(attribute) {
		return &list, fmt.Errorf("invalid attribute for describe")
	}

	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	result, err := svc.DescribeInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				slog.Error(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			slog.Error(err.Error())
		}
		return &list, err
	}

	var group = make([][]Dimension, 0)
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			dims := make([]Dimension, 1)
			r := reflect.ValueOf(instance)
			f := reflect.Indirect(r).FieldByName(attribute)
			dims[0] = Dimension{Name: attribute, Value: f.Elem().String()}
			group = append(group, dims)
		}
	}
	list.Groups = group
	return &list, err
}
