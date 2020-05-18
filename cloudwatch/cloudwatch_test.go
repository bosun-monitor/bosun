package cloudwatch

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"strconv"
	"sync"
	"testing"
	"time"
)

const namespace = "AWS/Kafka"
const metric = "CpuSystem"
const region = "eu-west-1"
const profile = "default"
const largeCluster = 95
const smallCluster = 15
const expansionLimit = 100
const pagesLimit = 10

// Singleton in real function prevents injection of appropriate mocks
func MockGetContextWithProvider(p ProfileProvider) Context {
	context = &cloudWatchContext{
		profileProvider: p,
		profiles:        make(map[string]cloudwatchiface.CloudWatchAPI),
		ExpansionLimit:  expansionLimit,
		PagesLimit:      pagesLimit,
	}
	return context
}

type slowProfileProvider struct {
	callCount int
}

func (s *slowProfileProvider) NewProfile(name, region string) cloudwatchiface.CloudWatchAPI {
	s.callCount += 1
	time.Sleep(3 * time.Second)
	return &cloudwatch.CloudWatch{}
}

func TestGetProfilOnlyCalledOnce(t *testing.T) {
	wg := sync.WaitGroup{}
	provider := &slowProfileProvider{}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, _ := GetContextWithProvider(provider).(*cloudWatchContext)
			ctx.getProfile("fake-profile", "fake-region")
		}()
	}

	wg.Wait()

	if provider.callCount != 1 {
		t.Errorf("Expected one call to NewProfile, got %d", provider.callCount)
	}
}

type mockProfileProvider struct {
}

type mockCloudWatchClient struct {
	cloudwatchiface.CloudWatchAPI
}

func (m *mockProfileProvider) NewProfile(name, region string) cloudwatchiface.CloudWatchAPI {
	return &mockCloudWatchClient{}
}

func (c mockCloudWatchClient) ListMetricsPages(li *cloudwatch.ListMetricsInput, callback func(*cloudwatch.ListMetricsOutput, bool) bool) error {
	var metrics []*cloudwatch.Metric
	var n = metric
	var ns = namespace

	clusters := make(map[string]int)
	if li.Dimensions == nil || (li.Dimensions != nil && *li.Dimensions[0].Value == "big") {
		clusters["big"] = largeCluster
	}

	if li.Dimensions == nil || (li.Dimensions != nil && *li.Dimensions[0].Value == "small") {
		clusters["small"] = smallCluster
	}

	for name, size := range clusters {
		for i := 0; i < size; i++ {
			dn := "Broker ID"
			dv := strconv.Itoa(i)
			dim := cloudwatch.Dimension{
				Name:  &dn,
				Value: &dv,
			}
			dimensions := []*cloudwatch.Dimension{&dim}

			cn := "Cluster Name"
			cv := name
			cdim := cloudwatch.Dimension{
				Name:  &cn,
				Value: &cv,
			}
			dimensions = append(dimensions, &cdim)
			metric := cloudwatch.Metric{
				Dimensions: dimensions,
				MetricName: &n,
				Namespace:  &ns,
			}
			metrics = append(metrics, &metric)
		}

		// Some aws metrics are logged with varying number of dimensions, to differentiate between cluster
		// level and node level values. The below adds a cluster only metric to test this case
		cn := "Cluster Name"
		cv := name
		dimensions := []*cloudwatch.Dimension{&cloudwatch.Dimension{
			Name:  &cn,
			Value: &cv,
		}}
		metric := cloudwatch.Metric{
			Dimensions: dimensions,
			MetricName: &n,
			Namespace:  &ns,
		}
		metrics = append(metrics, &metric)
	}

	lmo := &cloudwatch.ListMetricsOutput{
		Metrics:   metrics,
		NextToken: nil,
	}
	callback(lmo, true)
	return nil
}

func (c mockCloudWatchClient) GetMetricData(input *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	var mdr []*cloudwatch.MetricDataResult
	cwo := &cloudwatch.GetMetricDataOutput{
		Messages:          nil,
		MetricDataResults: mdr,
		NextToken:         nil,
	}

	if len(input.MetricDataQueries) == 0 {
		return cwo, nil
	}

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("q{i}")
		m := cloudwatch.MetricDataResult{
			Id:         &id,
			Label:      nil,
			Messages:   nil,
			StatusCode: nil,
			Timestamps: nil,
			Values:     nil,
		}

		mdr = append(mdr, &m)
	}

	cwo.MetricDataResults = mdr
	return cwo, nil
}

// Mocks to simulate being rate limited and test error handling

type rateLimitedProfileProvider struct {
}
type mockCloudWatchRateLimitedClient struct {
	cloudwatchiface.CloudWatchAPI
}

func (m *rateLimitedProfileProvider) NewProfile(name, region string) cloudwatchiface.CloudWatchAPI {
	return &mockCloudWatchRateLimitedClient{}
}

func (m *mockCloudWatchRateLimitedClient) GetMetricData(input *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	e := fmt.Errorf("Rate Limit exceeded")
	ae := awserr.New("429", "Rate Limited Exceeded", e)
	return nil, awserr.NewRequestFailure(ae, 429, "a5442de54s5454")
}

func (c mockCloudWatchRateLimitedClient) ListMetricsPages(li *cloudwatch.ListMetricsInput, callback func(*cloudwatch.ListMetricsOutput, bool) bool) error {
	e := fmt.Errorf("Rate Limit exceeded")
	ae := awserr.New("429", "Rate Limited Exceeded", e)
	return awserr.NewRequestFailure(ae, 429, "a5442de54s5454")
}

// -----

// Mocks for checking paging behaviour

type pagingProfileProvider struct {
}
type mockCloudWatchPagingClient struct {
	cloudwatchiface.CloudWatchAPI
}

func (m *pagingProfileProvider) NewProfile(name, region string) cloudwatchiface.CloudWatchAPI {
	return &mockCloudWatchPagingClient{}
}

func (c mockCloudWatchPagingClient) ListMetricsPages(li *cloudwatch.ListMetricsInput, callback func(*cloudwatch.ListMetricsOutput, bool) bool) error {
	var metrics []*cloudwatch.Metric
	lmo := &cloudwatch.ListMetricsOutput{
		Metrics:   metrics,
		NextToken: nil,
	}
	p := 0
	for callback(lmo, p == pagesLimit) {
		p++
	}
	return nil
}

// ----------------------------

func TestLookupDimensions(t *testing.T) {
	c := MockGetContextWithProvider(&mockProfileProvider{})

	lr := LookupRequest{
		Region:     region,
		Namespace:  namespace,
		Metric:     metric,
		Dimensions: nil,
		Profile:    profile,
	}

	var tests = []struct {
		dims  [][]Dimension
		count int
		e     error
	}{
		{[][]Dimension{{
			Dimension{
				Name:  "Broker ID",
				Value: "*",
			}, Dimension{
				Name:  "Cluster Name",
				Value: "*",
			},
		}}, 0, ErrExpansionLimit},
		{[][]Dimension{{
			Dimension{
				Name:  "Broker ID",
				Value: "*",
			}, Dimension{
				Name:  "Cluster Name",
				Value: "big",
			},
		}}, largeCluster, nil},
		{[][]Dimension{{
			Dimension{
				Name:  "Broker ID",
				Value: "*",
			}, Dimension{
				Name:  "Cluster Name",
				Value: "small",
			},
		}}, smallCluster, nil},
		{[][]Dimension{{
			Dimension{
				Name:  "Irrelevant Dimension",
				Value: "1234",
			}, Dimension{
				Name:  "Cluster Name",
				Value: "small",
			},
		}}, 0, nil},
	}
	for _, test := range tests {
		lr.Dimensions = test.dims
		res, err := c.LookupDimensions(&lr)

		if err != test.e {
			t.Error(err)
		}
		if len(res) != test.count {
			t.Errorf("Did not get expected count, wanted %d got %d", test.count, len(res))
		}
	}

}

func TestLookupPageLimit(t *testing.T) {
	c := MockGetContextWithProvider(&pagingProfileProvider{})

	lr := LookupRequest{
		Region:     region,
		Namespace:  namespace,
		Metric:     metric,
		Dimensions: nil,
		Profile:    profile,
	}

	_, err := c.LookupDimensions(&lr)
	if err != ErrPagingLimit {
		t.Error("Should have failed from hitting expansion limit")
	}
}

func TestLookupDimensionsError(t *testing.T) {
	c := MockGetContextWithProvider(&rateLimitedProfileProvider{})
	dims := [][]Dimension{{
		Dimension{
			Name:  "Broker ID",
			Value: "*",
		}, Dimension{
			Name:  "Cluster Name",
			Value: "*",
		}}}

	lr := LookupRequest{
		Region:     region,
		Namespace:  namespace,
		Metric:     metric,
		Dimensions: dims,
		Profile:    profile,
	}
	_, err := c.LookupDimensions(&lr)
	if err == nil {
		t.Error("Error did not bubble up correctly")
	}
}

func TestQuery(t *testing.T) {
	c := MockGetContextWithProvider(&mockProfileProvider{})
	start := time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2018, time.January, 1, 1, 0, 0, 0, time.UTC)

	dims := [][]Dimension{{
		Dimension{
			Name:  "Broker ID",
			Value: "*",
		}, Dimension{
			Name:  "Cluster Name",
			Value: "grappler-msk-A",
		}}}

	tests := []struct {
		r    Request
		err  error
		size int
	}{
		{
			r: Request{
				Start:      &start,
				End:        &end,
				Region:     region,
				Namespace:  namespace,
				Metric:     metric,
				Period:     60,
				Statistic:  "Sum",
				Dimensions: dims,
				Profile:    profile,
			},
			err:  nil,
			size: 10,
		},
		{
			r: Request{
				Start:      &start,
				End:        &end,
				Region:     region,
				Namespace:  namespace,
				Metric:     metric,
				Period:     60,
				Statistic:  "Sum",
				Dimensions: nil,
				Profile:    profile,
			},
			err:  nil,
			size: 10,
		},
		{
			r: Request{
				Start:      &start,
				End:        &end,
				Region:     region,
				Namespace:  namespace,
				Metric:     metric,
				Period:     0,
				Statistic:  "Sum",
				Dimensions: nil,
				Profile:    profile,
			},
			err:  ErrInvalidPeriod,
			size: 0,
		},
	}
	for _, test := range tests {
		res, err := c.Query(&test.r)
		if err != test.err {
			t.Errorf("Query failed, expect error to be %v, got %v", test.err, err)
		}
		if len(res.Raw.MetricDataResults) != test.size {
			t.Errorf("Query returned wrong number of results, expected %d, got %d", test.size, len(res.Raw.MetricDataResults))
		}
	}

}

func TestQueryError(t *testing.T) {
	c := MockGetContextWithProvider(&rateLimitedProfileProvider{})
	start := time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2018, time.January, 1, 1, 0, 0, 0, time.UTC)

	dims := [][]Dimension{{
		Dimension{
			Name:  "Broker ID",
			Value: "*",
		}, Dimension{
			Name:  "Cluster Name",
			Value: "grappler-msk-A",
		}}}

	r := Request{
		Start:      &start,
		End:        &end,
		Region:     region,
		Namespace:  namespace,
		Metric:     metric,
		Period:     60,
		Statistic:  "Sum",
		Dimensions: dims,
		Profile:    profile,
	}
	_, err := c.Query(&r)
	if err == nil {
		t.Error("Error did not bubble properly", err)
	}

}

func TestFilterDimensions(t *testing.T) {

	metric := "FreeableMemory"
	namespace := "AWS/ElastiCache"

	d1 := "CacheClusterId"
	v1 := "grappler-cluster-1"

	d2 := "CacheNodeId"
	v2 := "0001"

	v3 := "not-cluster-1"

	wildcards := make(Wildcards)
	wildcards[d1] = "grappler-cluster-1"
	wildcards[d2] = "0*"

	// set dimensions that are present in the query and we expect to be in results set
	ds := make(DimensionSet)
	ds[d1] = true
	ds[d2] = true

	// example of elasticache node level metric
	metric1 := cloudwatch.Metric{
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  &d1,
				Value: &v1,
			},
			{
				Name:  &d2,
				Value: &v2,
			},
		},
		MetricName: &metric,
		Namespace:  &namespace,
	}

	// cluster level metric
	metric2 := cloudwatch.Metric{

		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  &d1,
				Value: &v1,
			}},
		MetricName: &metric,
		Namespace:  &namespace,
	}

	// account level metric
	metric3 := cloudwatch.Metric{

		Dimensions: nil,
		MetricName: &metric,
		Namespace:  &namespace,
	}

	// different cluster than the one we're searching for
	metric4 := cloudwatch.Metric{
		Dimensions: []*cloudwatch.Dimension{
			{
				Name:  &d1,
				Value: &v3,
			},
			{
				Name:  &d2,
				Value: &v2,
			},
		},
		MetricName: &metric,
		Namespace:  &namespace,
	}

	metrics := []*cloudwatch.Metric{&metric1, &metric2, &metric3, &metric4}

	m, err := filterDimensions(metrics, wildcards, ds, expansionLimit)
	if err != nil {
		t.Error(err)
	}
	// only  the node level metric should match the filter criteria
	if len(m) != 1 || m[0][0].Value != v1 || m[0][1].Value != v2 {
		t.Error("Filter didn't select correct metric")
	}

}

func TestCacheKeyMatch(t *testing.T) {
	start := time.Date(2018, 7, 4, 17, 0, 0, 0, time.UTC)
	end := time.Date(2018, 7, 4, 18, 0, 0, 0, time.UTC)
	var tests = []struct {
		req Request
		key string
	}{
		{req: Request{
			Start:     &start,
			End:       &end,
			Region:    "eu-west-1",
			Namespace: "AWS/EC2",
			Metric:    "CPUUtilization",
			Period:    60, Statistic: "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
			key: "cloudwatch-1530723600-1530727200-eu-west-1-AWS/EC2-CPUUtilization-60-Sum-InstanceId:i-0106b4d25c54baac7-prod"},
	}

	for _, u := range tests {
		calculatedKey := u.req.CacheKey()
		if u.key != calculatedKey {
			t.Errorf("Cache key doesn't match, expected '%s' got '%s' ", u.key, calculatedKey)
		}
	}

}

func TestCacheKeyMisMatch(t *testing.T) {

	start := time.Date(2018, 7, 4, 17, 0, 0, 0, time.UTC)
	end := time.Date(2018, 7, 4, 18, 0, 0, 0, time.UTC)
	exampleRequest := Request{
		Start:           &start,
		End:             &end,
		Region:          "eu-west-1",
		Namespace:       "AWS/EC2",
		Metric:          "CPUUtilization",
		Period:          60,
		Statistic:       "Sum",
		DimensionString: "InstanceId:i-0106b4d25c54baac7",
		Profile:         "prod",
	}

	exampleKey := exampleRequest.CacheKey()

	variantStart := time.Date(2018, 7, 4, 17, 30, 0, 0, time.UTC)
	variantEnd := time.Date(2018, 7, 4, 18, 30, 0, 0, time.UTC)
	var tests = []Request{
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25*",
			Profile:         "prod",
		},
		{
			Start:           &variantStart,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &variantEnd,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-central-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/ECS",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "MemoryUsage",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          300,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Avg",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          300,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-01064646d6d6baac7",
			Profile:         "prod",
		},
		{
			Start:           &start,
			End:             &end,
			Region:          "eu-west-1",
			Namespace:       "AWS/EC2",
			Metric:          "CPUUtilization",
			Period:          60,
			Statistic:       "Sum",
			DimensionString: "InstanceId:i-0106b4d25c54baac7",
			Profile:         "sandbox",
		},
	}
	for _, u := range tests {
		calculatedKey := u.CacheKey()
		if exampleKey == calculatedKey {
			t.Errorf("Calculated key shouldn't match example but does. '%s' == '%s' ", calculatedKey, exampleKey)
		}
	}
}
