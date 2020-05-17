package collectors

import (
	"fmt"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
)

const (
	awsCPU                = "aws.ec2.cpu"
	awsEC2DiskBytes       = "aws.ec2.disk.bytes"
	awsEC2DiskOps         = "aws.ec2.disk.ops"
	awsELBHostsHealthy    = "aws.elb.hosts.healthy"
	awsELBHostsUnHealthy  = "aws.elb.hosts.unhealthy"
	awsELBLatencyAvg      = "aws.elb.latency.average"
	awsELBLatencyMax      = "aws.elb.latency.maximum"
	awsELBLatencyMin      = "aws.elb.latency.minimum"
	awsNetwork            = "aws.ec2.net.bytes"
	awsStatusCheckFailed  = "aws.ec2.status.failed"
	descAWSEC2CPU         = "The average CPU Utilization, gathered at a 60 second interval and averaged over five minutes."
	descAWSEC2DiskBytes   = "The average bytes written or read via disk, gathered at a 60 second interval and averaged over five minutes."
	descAWSEC2DiskOps     = "The average disk operations, either written or read, gathered at a 60 second interval and averaged over five minutes."
	descAWSEC2NetBytes    = "The average bytes transmitted or received via network, gathered at a 60 second interval and averaged over five minutes."
	descAWSEC2StatusCheck = "The EC2 Status Check, which includes both instance-level and system-level drill-down, gathered every 60 seconds."
	descAWSELBHostCount   = "The number of instances in what the Elastic Load Balancer considers a healthy state, gathered every 60 seconds."
	descAWSELBLatency     = "The minimum, maximum and average latency as reported by the load balancer, gathered at a 60 second interval and averaged over five minutes."
)

var aws_period = int64(60)

// AWS collects data from Amazon Web Services
func AWS(accessKey, secretKey, region, productCodes, bucketName, bucketPath string, purgeDays int) error {
	if accessKey == "" || secretKey == "" || region == "" {
		return fmt.Errorf("empty AccessKey, SecretKey, or Region in AWS")
	}
	//mhenderson: There are some alerts in the aws collector that we don't want to output in the event that
	//billing only is enabled, as you might enable billing without having any EC3 or ELB instances.
	billingEnabled := bucketName != "" && bucketPath != ""
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_aws(accessKey, secretKey, region, billingEnabled)
		},
		Interval: 60 * time.Second,
		name:     fmt.Sprintf("aws-%s", region),
	})

	if billingEnabled {
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_awsBilling(accessKey, secretKey, region, productCodes, bucketName, bucketPath, purgeDays)
			},
			Interval: 1 * time.Hour,
			name:     fmt.Sprintf("awsBilling-%s", region),
		})
	}
	return nil
}

func c_aws(accessKey, secretKey, region string, billingEnabled bool) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	conf := &aws.Config{
		Credentials: creds,
		Region:      &region,
	}
	ecc := ec2.New(session.New(), conf)
	if ecc == nil {
		return nil, fmt.Errorf("unable to login to EC2")
	}
	elb := elb.New(session.New(), conf)
	if elb == nil {
		return nil, fmt.Errorf("unable to login to ELB")
	}
	cw := cloudwatch.New(session.New(), conf)
	if cw == nil {
		return nil, fmt.Errorf("unable to login to CloudWatch")
	}
	instances, err := awsGetInstances(*ecc)
	if err != nil && !billingEnabled {
		slog.Warning("No EC2 Instances found.")
	}
	loadBalancers, err := awsGetLoadBalancers(*elb)
	if err != nil && !billingEnabled {
		slog.Warning("No ELB Load Balancers found.")
	}
	for _, loadBalancer := range loadBalancers {
		awsGetELBLatency(*cw, &md, loadBalancer)
		awsGetELBHostCounts(*cw, &md, loadBalancer)
	}
	for _, instance := range instances {
		awsGetCPU(*cw, &md, instance)
		awsGetNetwork(*cw, &md, instance)
		awsGetDiskBytes(*cw, &md, instance)
		awsGetDiskOps(*cw, &md, instance)
		awsGetStatusChecks(*cw, &md, instance)
	}
	return md, nil
}

func awsGetInstances(ecc ec2.EC2) ([]*ec2.Instance, error) {
	instancelist := []*ec2.Instance{}
	resp, err := ecc.DescribeInstances(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to describe EC2 Instances")
	}
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			instancelist = append(instancelist, instance)
		}
	}
	return instancelist, nil
}

func awsGetLoadBalancers(lb elb.ELB) ([]*elb.LoadBalancerDescription, error) {
	lbList := []*elb.LoadBalancerDescription{}
	resp, err := lb.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to describe ELB Balancers")
	}
	for _, loadBalancer := range resp.LoadBalancerDescriptions {
		lbList = append(lbList, loadBalancer)
	}
	return lbList, nil
}

func awsGetCPU(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance *ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -600)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("CPUUtilization"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Percent"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceId}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	tags := opentsdb.TagSet{
		"instance": *instance.InstanceId,
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsCPU, datapoint.Timestamp.Unix(), *datapoint.Average, tags, metadata.Gauge, metadata.Pct, descAWSEC2CPU)
	}
	return nil
}
func awsGetNetwork(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance *ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -600)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("NetworkIn"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Bytes"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceId}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsNetwork, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "direction": "in"}, metadata.Gauge, metadata.Bytes, descAWSEC2NetBytes)
	}
	search.MetricName = aws.String("NetworkOut")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsNetwork, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "direction": "out"}, metadata.Gauge, metadata.Bytes, descAWSEC2NetBytes)
	}
	return nil
}

func awsGetDiskBytes(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance *ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -600)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("DiskReadBytes"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Bytes"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceId}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskBytes, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "operation": "read"}, metadata.Gauge, metadata.Bytes, descAWSEC2DiskBytes)
	}
	search.MetricName = aws.String("DiskWriteBytes")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskBytes, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "operation": "write"}, metadata.Gauge, metadata.Bytes, descAWSEC2DiskBytes)
	}
	return nil
}

func awsGetDiskOps(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance *ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -600)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("DiskReadOps"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Count"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceId}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskOps, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "operation": "read"}, metadata.Gauge, metadata.Count, descAWSEC2DiskOps)
	}
	search.MetricName = aws.String("DiskWriteOps")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskOps, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "operation": "write"}, metadata.Gauge, metadata.Count, descAWSEC2DiskOps)
	}
	return nil
}

func awsGetStatusChecks(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance *ec2.Instance) error {
	period := int64(60)
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -60)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("StatusCheckFailed"),
		Period:     &period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Count"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceId}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId}, metadata.Gauge, metadata.Count, descAWSEC2StatusCheck)
	}
	search.MetricName = aws.String("StatusCheckFailed_Instance")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "category": "instance"}, metadata.Gauge, metadata.Count, descAWSEC2StatusCheck)
	}
	search.MetricName = aws.String("StatusCheckFailed_System")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceId, "category": "system"}, metadata.Gauge, metadata.Count, descAWSEC2StatusCheck)
	}
	return nil
}

func awsGetELBLatency(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, loadBalancer *elb.LoadBalancerDescription) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -4000)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("Latency"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average"), aws.String("Minimum"), aws.String("Maximum")},
		Namespace:  aws.String("AWS/ELB"),
		Unit:       aws.String("Seconds"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("LoadBalancerName"), Value: loadBalancer.LoadBalancerName}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsELBLatencyMin, datapoint.Timestamp.Unix(), *datapoint.Minimum, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, descAWSELBLatency)
		AddTS(md, awsELBLatencyMax, datapoint.Timestamp.Unix(), *datapoint.Maximum, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, descAWSELBLatency)
		AddTS(md, awsELBLatencyAvg, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, descAWSELBLatency)
	}
	return nil
}
func awsGetELBHostCounts(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, loadBalancer *elb.LoadBalancerDescription) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  aws.Time(time.Now().UTC().Add(time.Second * -60)),
		EndTime:    aws.Time(time.Now().UTC()),
		MetricName: aws.String("HealthyHostCount"),
		Period:     &aws_period,
		Statistics: []*string{aws.String("Average")},
		Namespace:  aws.String("AWS/ELB"),
		Unit:       aws.String("Count"),
		Dimensions: []*cloudwatch.Dimension{{Name: aws.String("LoadBalancerName"), Value: loadBalancer.LoadBalancerName}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsELBHostsHealthy, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, descAWSELBHostCount)
	}
	search.MetricName = aws.String("UnhealthyHostCount")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return err
	}
	if resp.Datapoints == nil {
		AddTS(md, awsELBHostsUnHealthy, time.Now().UTC().Unix(), 0, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, descAWSELBHostCount)
	} else {
		for _, datapoint := range resp.Datapoints {
			AddTS(md, awsELBHostsUnHealthy, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadBalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, descAWSELBHostCount)
		}
	}
	return nil
}
