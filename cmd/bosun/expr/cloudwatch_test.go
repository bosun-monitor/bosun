package expr

import (
	"strconv"
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
	c := cloudwatch.Config{}
	svc := new(mockCloudWatchClient)
	c.Svc = svc
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

	var tests = []struct {
		region     string
		namespace  string
		metric     string
		period     string
		statistics string
		dimensions string
		start      string
		end        string
	}{
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Sum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Average", "InstanceId:i-0106b4d25c54baac7", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Maximum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h"},
		{"eu-west-1", "AWS/EC2", "CPUUtilization", "60", "Minimum", "InstanceId:i-0106b4d25c54baac7", "2h", "1h"},
	}
	for _, u := range tests {

		results, err := CloudWatchQuery(&e, new(miniprofiler.Profile), u.region,
			u.namespace, u.metric, u.period, u.statistics,
			u.dimensions, u.start, u.end)
		if err != nil {
			t.Errorf("Query Failure: %s ", err)
		} else if results.Results[0].Group.String() != "{InstanceId=i-0106b4d25c54baac7}" {
			t.Errorf("Group mismatch")
		}
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
