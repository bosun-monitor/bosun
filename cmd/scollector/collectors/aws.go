package collectors

import (
	"fmt"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	//"github.com/awslabs/aws-sdk-go/gen/elb"
	"github.com/awslabs/aws-sdk-go/gen/cloudwatch"
)

const (
	awsCPU string = "aws.ec2.cpu"
)

func AWS(accessKey, secretKey, region string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_aws(accessKey, secretKey, region)
		},
		name: fmt.Sprintf("aws-%s", region),
	})
}

func c_aws(accessKey, secretKey, region string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	creds := aws.Creds(accessKey, secretKey, "")
	ecc := ec2.New(creds, region, nil)
	//elb := elb.New(creds, region, nil)
	cw := cloudwatch.New(creds, region, nil)

	instances, err := AWSGetInstances(*ecc)
	if err != false {
		// Do something useful in error
	}
	for _, instance := range instances {
		AWSGetCPU(*cw, &md, instance)
	}
	return md, nil

}

func AWSGetInstances(ecc ec2.EC2) ([]ec2.Instance, bool) {
	instancelist := []ec2.Instance{}
	resp, err := ecc.DescribeInstances(nil)
	if err != nil {
		return nil, true
	}

	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			instancelist=append(instancelist, instance)
		}
	}
	return instancelist, false
}

func AWSGetCPU(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().Add(time.Second * -60),
		EndTime:    time.Now(),
		MetricName: aws.String("CPUUtilization"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Percent"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		slog.Warning(err)
	}
	tags := opentsdb.TagSet{
		"instance": *instance.InstanceID,
	}

	for _, datapoint := range resp.Datapoints {
		Add(md, awsCPU, *datapoint.Average, tags, metadata.Gauge, metadata.Pct, "")
	}
}
