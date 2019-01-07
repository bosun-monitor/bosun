package expr

import (
	"bosun.org/cmd/bosun/expr/parse"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"strconv"
	"testing"
	"time"

	"bosun.org/cloudwatch"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

const TestInstanceID = "i-123455ab"

type mockCloudWatchClient struct {
	cloudwatchiface.CloudWatchAPI
}

func (m *mockCloudWatchClient) GetMetricStatistics(input *cw.GetMetricStatisticsInput) (output *cw.GetMetricStatisticsOutput, err error) {
	output = new(cw.GetMetricStatisticsOutput)
	datapoints := make([]*cw.Datapoint, 0)
	for i := 10; i >= 0; i-- {
		t := time.Now()
		dur, _ := time.ParseDuration(strconv.Itoa(i*60) + "s")
		t = t.Add(-dur)
		datapoint, _ := buildDatapoint(&t)
		datapoints = append(datapoints, datapoint)
	}

	output.Label = input.MetricName
	output.Datapoints = datapoints
	return output, nil
}

type mockEC2Client struct {
	ec2iface.EC2API
}

func (m *mockEC2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	var instance ec2.Instance
	instanceId := TestInstanceID
	instance.InstanceId = &instanceId
	var reservation ec2.Reservation
	reservation.Instances = append(reservation.Instances, &instance)
	reservations := []*ec2.Reservation{&reservation}
	dio := ec2.DescribeInstancesOutput{Reservations: reservations}
	return &dio, nil
}

func buildDatapoint(t *time.Time) (point *cw.Datapoint, err error) {
	var sum, average, maximum, minimum, sampleCount float64
	average = 1
	sum = 1
	maximum = 1
	minimum = 1
	sampleCount = 1

	timestamp := t
	d := new(cw.Datapoint)
	d.Average = &average
	d.Maximum = &maximum
	d.Minimum = &minimum
	d.SampleCount = &sampleCount
	d.Sum = &sum
	d.Timestamp = timestamp
	return d, nil
}

func TestCloudWatchQuery(t *testing.T) {
	c := cloudwatch.NewConfig(1)
	svc := new(mockCloudWatchClient)
	c.CWProfiles["bosun-default"] = svc
	e := State{
		now: time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC),
		Backends: &Backends{
			CloudWatchContext: c,
		},
		BosunProviders: &BosunProviders{
			Squelched: func(tags opentsdb.TagSet) bool {
				return false
			},
		},
		Timer: new(miniprofiler.Profile),
	}

	var tests = []struct {
		region     string
		namespace  string
		metric     string
		period     float64
		statistics string
		queries    string
		start      string
		end        string
	}{
		{"eu-west-1", "AWS/EC2", "CPUUtilization", 60, "Sum", "[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", 60, "Average", "[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", 60, "Maximum", "[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", 60, "Minimum", "[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", 60, "SampleCount", "[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", "2h", "1h"},
	}
	for _, u := range tests {
		dl := cloudwatch.DimensionList{}
		groups, err := parseDimensionList(u.queries)
		if err != nil {
			t.Error(err)
		}
		dl.Groups = groups

		results, err := CloudWatchQuery("default", &e, u.region,
			u.namespace, u.metric, u.period, u.statistics, dl, u.start, u.end)
		if err != nil {
			t.Errorf("Query Failure: %s ", err)
		} else if len(results.Results) != 1 || results.Results[0].Group.String() != "{InstanceId=i-0106b4d25c54baac7}" {
			t.Errorf("Group mismatch")
		}
	}
}

func buildQueryTree() []parse.Node {
	args := make([]parse.Node, 8)
	args[0] = new(parse.StringNode)
	args[1] = new(parse.StringNode)
	args[2] = new(parse.StringNode)
	args[3] = new(parse.NumberNode)
	args[4] = new(parse.StringNode)
	args[6] = new(parse.StringNode)
	args[7] = new(parse.StringNode)
	return args
}

func TestCloudWatchTagQueryDim(t *testing.T) {
	var tests = []struct {
		dimensions string
		tags       parse.Tags
	}{
		{"[{\"InstanceId\":\"i-0106b4d25c54baac7\"}]", parse.Tags{"InstanceId": {}}},
		{"[{\"InstanceId\":\"i-0106b4d25c54baac7\"},{\"AutoScalingGroupName\":\"asg123\"}]", parse.Tags{"AutoScalingGroupName": {}, "InstanceId": {}}},
		{"", parse.Tags{}},
	}

	args := buildQueryTree()

	for _, u := range tests {
		fn := new(parse.FuncNode)
		fn.Name = "cwdim"
		sn := new(parse.StringNode)
		sn.Text = u.dimensions
		fn.Args = append(fn.Args, sn)
		args[5] = fn

		tags, err := cloudwatchTagQuery(args)
		if err != nil {
			t.Errorf("Error parsing tags %s", err)
		}
		if !tags.Equal(u.tags) {
			t.Errorf("Missmatching tags, expected '%s' , got '%s' ", u.tags, tags)
		}
	}
}

func TestCloudWatchTagQueryDimQ(t *testing.T) {
	var tests = []struct {
		region    string
		namespace string
		dimension string
		query     string
		tags      parse.Tags
	}{
		{"eu-west-1", "EC2", "InstanceId", "[{\"tag:Project\":[\"Grappler\"]}]", parse.Tags{"InstanceId": {}}},
	}

	args := buildQueryTree()

	for _, u := range tests {
		fn := new(parse.FuncNode)
		fn.Name = "cwdimq"

		a := make([]parse.Node, 4)

		r := new(parse.StringNode)
		r.Text = u.region
		a[0] = r

		n := new(parse.StringNode)
		n.Text = u.namespace
		a[1] = n

		d := new(parse.StringNode)
		d.Text = u.dimension
		a[2] = d

		q := new(parse.StringNode)
		q.Text = u.query
		a[3] = q

		fn.Args = a

		args[5] = fn

		tags, err := cloudwatchTagQuery(args)
		if err != nil {
			t.Errorf("Error parsing tags %s", err)
		}
		if !tags.Equal(u.tags) {
			t.Errorf("Missmatching tags, expected '%s' , got '%s' ", u.tags, tags)
		}
	}
}

func TestCacheKeyMatch(t *testing.T) {
	start := time.Date(2018, 7, 4, 17, 0, 0, 0, time.UTC)
	end := time.Date(2018, 7, 4, 18, 0, 0, 0, time.UTC)
	var tests = []struct {
		req cloudwatch.Request
		key string
	}{
		{req: cloudwatch.Request{
			Start:     &start,
			End:       &end,
			Region:    "eu-west-1",
			Namespace: "AWS/EC2",
			Metric:    "CPUUtilization",
			Period:    60, Statistic: "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
			key: "cloudwatch-1530723600-1530727200-eu-west-1-AWS/EC2-CPUUtilization-60-Sum-[InstanceId:i-0106b4d25c54baac7]-prod"},
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
	exampleRequest := cloudwatch.Request{
		Start:      &start,
		End:        &end,
		Region:     "eu-west-1",
		Namespace:  "AWS/EC2",
		Metric:     "CPUUtilization",
		Period:     60,
		Statistic:  "Sum",
		Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
		Profile:    "prod",
	}

	exampleKey := exampleRequest.CacheKey()

	variantStart := time.Date(2018, 7, 4, 17, 30, 0, 0, time.UTC)
	variantEnd := time.Date(2018, 7, 4, 18, 30, 0, 0, time.UTC)
	var tests = []cloudwatch.Request{
		{
			Start:      &variantStart,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &variantEnd,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-central-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/ECS",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "MemoryUsage",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     300,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Avg",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     300,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-01064646d6d6baac7"}},
			Profile:    "prod",
		},
		{
			Start:      &start,
			End:        &end,
			Region:     "eu-west-1",
			Namespace:  "AWS/EC2",
			Metric:     "CPUUtilization",
			Period:     60,
			Statistic:  "Sum",
			Dimensions: []cloudwatch.Dimension{{Name: "InstanceId", Value: "i-0106b4d25c54baac7"}},
			Profile:    "sandbox",
		},
	}
	for _, u := range tests {
		calculatedKey := u.CacheKey()
		if exampleKey == calculatedKey {
			t.Errorf("Calculated key shouldn't match example but does. '%s' == '%s' ", calculatedKey, exampleKey)
		}
	}
}

func TestBuildDimensions(t *testing.T) {

	c := cloudwatch.NewConfig(1)
	svc := new(mockCloudWatchClient)
	c.CWProfiles["bosun-default"] = svc
	e := State{
		now: time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC),
		Backends: &Backends{
			CloudWatchContext: c,
		},
		BosunProviders: &BosunProviders{
			Squelched: func(tags opentsdb.TagSet) bool {
				return false
			},
		},
	}

	results, err := CloudWatchBuildDimensions("default", &e, "[{\"InstanceId\":\"i-123455\",\"AutoScalingGroup\":\"abcde\"},{\"InstanceId\":\"i-5678\",\"AutoScalingGroup\":\"fghij\"}]")
	if err != nil {
		t.Errorf("Error building dimensions %s", err)
	}
	if results == nil {
		t.Errorf("Dimensions were empty %s", err)

	}
}

func TestEC2Describe(t *testing.T) {
	c := cloudwatch.NewConfig(1)
	svc := new(mockEC2Client)
	c.EC2Profiles["user-prod"] = svc
	e := State{
		now: time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC),
		Backends: &Backends{
			CloudWatchContext: c,
		},
		BosunProviders: &BosunProviders{
			Squelched: func(tags opentsdb.TagSet) bool {
				return false
			},
		},
	}

	res, err := CloudWatchQueryDimensions("prod", &e, "eu-west-1", "EC2", "InstanceId", "[{\"tag:Stack\":[\"Webservers\"]}]")
	if err != nil {
		t.Errorf("Error querying dimensions %s", err)

	}

	if res != nil {
		dl := res.Results[0].Value.(*cloudwatch.DimensionList)
		if dl.Groups[0][0].Name != "InstanceId" || dl.Groups[0][0].Value != TestInstanceID {
			t.Errorf("Invalid Response")
		}
	}

}
