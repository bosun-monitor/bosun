package expr

import (
	"testing"
	"time"

	"bosun.org/cloudwatch"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	cw "github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

type mockCloudWatchClient struct {
	cloudwatchiface.CloudWatchAPI
}

type mockProfileProvider struct{}

func (m *mockProfileProvider) NewProfile(name, region string) cloudwatchiface.CloudWatchAPI {
	return &mockCloudWatchClient{}
}

const metric = "CPUUtilzation"
const namespace = "AWS/EC2"

func (c mockCloudWatchClient) ListMetricsPages(li *cw.ListMetricsInput, callback func(*cw.ListMetricsOutput, bool) bool) error {
	var metrics []*cw.Metric
	var n = metric
	var ns = namespace

	var instances []string
	if li.Dimensions == nil || (li.Dimensions != nil && *li.Dimensions[0].Value == "0106b4d25c54baac7") {
		instances = append(instances, "0106b4d25c54baac7")
	}

	if li.Dimensions == nil {
		instances = append(instances, "5306b4d25c546577l")
		instances = append(instances, "910asdasd25c5477l")
	}

	for _, inst := range instances {
		dn := "InstanceId"
		dv := inst
		dim := cw.Dimension{
			Name:  &dn,
			Value: &dv,
		}
		dimensions := []*cw.Dimension{&dim}
		metric := cw.Metric{
			Dimensions: dimensions,
			MetricName: &n,
			Namespace:  &ns,
		}
		metrics = append(metrics, &metric)
	}

	lmo := &cw.ListMetricsOutput{
		Metrics:   metrics,
		NextToken: nil,
	}

	callback(lmo, false)
	return nil
}

func (m *mockCloudWatchClient) GetMetricData(cwi *cw.GetMetricDataInput) (*cw.GetMetricDataOutput, error) {
	var mdr []*cw.MetricDataResult
	var r cw.MetricDataResult
	var timestamps []*time.Time
	var values []*float64

	for _, mdq := range cwi.MetricDataQueries {
		for j := 0; j < 600; j = j + int(*mdq.MetricStat.Period) {
			time := cwi.StartTime.Add(time.Second * time.Duration(j))
			timestamps = append(timestamps, &time)
			val := float64(j)
			values = append(values, &val)
		}
		r = cw.MetricDataResult{
			Id:         mdq.Id,
			Label:      nil,
			Messages:   nil,
			StatusCode: nil,
			Timestamps: timestamps,
			Values:     values,
		}
		mdr = append(mdr, &r)
	}

	o := cw.GetMetricDataOutput{
		Messages:          nil,
		MetricDataResults: mdr,
		NextToken:         nil,
	}
	return &o, nil
}

func TestCloudWatchQuery(t *testing.T) {
	c := cloudwatch.GetContextWithProvider(&mockProfileProvider{})

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
		period     string
		statistics string
		dimensions string
		start      string
		end        string
		expected   string
	}{
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60s", "Sum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h", "{InstanceId=i-0106b4d25c54baac7}"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "1m", "Average", "InstanceId:i-0106b4d25c54baac7", "2h", "1h", "{InstanceId=i-0106b4d25c54baac7}"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Maximum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h", "{InstanceId=i-0106b4d25c54baac7}"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Minimum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h", "{InstanceId=i-0106b4d25c54baac7}"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Minimum", "InstanceId:*", "2h", "1h", "{InstanceId=910asdasd25c5477l}"},
	}
	for _, u := range tests {

		results, err := CloudWatchQuery("default", &e, u.region,
			u.namespace, u.metric, u.period, u.statistics,
			u.dimensions, u.start, u.end)

		if err != nil {
			t.Errorf("Query Failure: %s ", err)
		} else if results.Results[0].Group.String() != u.expected {
			t.Errorf("Group mismatch got %s , expected %s", results.Results[0].Group.String(), u.expected)
		}
	}
}

func TestDateParseFail(t *testing.T) {
	c := cloudwatch.GetContextWithProvider(&mockProfileProvider{})

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
		period string
		start  string
		end    string
		err    error
	}{
		{"60s", "2h", "1h", nil},
		{"60x", "2h", "1h", PeriodParseError},
		{"60s", "2x", "1h", StartParseError},
		{"60s", "2h", "1x", EndParseError},
	}
	for _, u := range tests {

		_, err := CloudWatchQuery("default", &e, "eu-west-1", "AWS/EC2", "CPUUtilization", u.period,
			"Sum", "InstanceId:i-0106b4d25c54baac7", u.start, u.end)

		if err != u.err {
			t.Errorf("Query Failure:  expected error to be %v, got %v", u.err, err)
		}
	}
}

func TestMultiRegion(t *testing.T) {
	c := cloudwatch.GetContextWithProvider(&mockProfileProvider{})

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
		dimension string
		expected  int
	}{
		{"eu-west-1", 1},
		{"eu-central-1,eu-west-1", 2},
		{"eu-west-1,eu-west-2,eu-central-1,ap-southeast-1", 4},
	}
	for _, u := range tests {

		res, err := CloudWatchQuery("default", &e, u.dimension, "AWS/EC2", "CPUUtilization", "1m",
			"Sum", "InstanceId:i-0106b4d25c54baac7", "1h", "")
		if err != nil {
			t.Errorf("Query Failure: %v", err)
		} else if len(res.Results) != u.expected {
			t.Errorf("Unexpected result set size, wanted %d, got %d results", u.expected, len(res.Results))
		}
	}
}

func TestCloudWatchQueryWithoutDimensions(t *testing.T) {
	c := cloudwatch.GetContextWithProvider(&mockProfileProvider{})
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

	results, err := CloudWatchQuery("default", &e, "eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Sum", " ", "2h", "1h")
	if err != nil {
		t.Errorf("Query Failure: %s ", err)
	} else if results.Results[0].Group.String() != "{}" {
		t.Errorf("Dimensions not parsed correctly, expected '%s' , got '%s' ", "{}", results.Results[0].Group.String())
	}
}
func TestCloudWatchTagQuery(t *testing.T) {
	var tests = []struct {
		dimensions string
		tags       parse.Tags
	}{
		{"InstanceId:i-0106b4d25c54baac7", parse.Tags{"InstanceId": {}}},
		{"InstanceId:i-0106b4d25c54baac7,AutoScalingGroupName:asg123", parse.Tags{"AutoScalingGroupName": {}, "InstanceId": {}}},
		{"", parse.Tags{}},
	}

	args := make([]parse.Node, 8)

	for _, u := range tests {
		n := new(parse.StringNode)
		n.Text = u.dimensions
		args[5] = n
		tags, err := cloudwatchTagQuery(args)
		if err != nil {
			t.Errorf("Error parsing tags %s", err)
		}
		if !tags.Equal(u.tags) {
			t.Errorf("Missmatching tags, expected '%s' , got '%s' ", u.tags, tags)
		}
	}
}
