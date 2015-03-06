package collectors

import (
	"fmt"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/cloudwatch"
	"github.com/awslabs/aws-sdk-go/gen/ec2"
	"github.com/awslabs/aws-sdk-go/gen/elb"
)

const (
	awsCPU                string = "aws.ec2.cpu"
	awsNetwork            string = "aws.ec2.net.bytes"
	awsEC2DiskBytes       string = "aws.ec2.disk.bytes"
	awsEC2DiskOps         string = "aws.ec2.disk.ops"
	awsStatusCheckFailed  string = "aws.ec2.status.failed"
	awsELBLatencyMin      string = "aws.elb.latency.minimum"
	awsELBLatencyMax      string = "aws.elb.latency.maximum"
	awsELBLatencyAvg      string = "aws.elb.latency.average"
	awsELBHostsHealthy    string = "aws.elb.hosts.healthy"
	awsELBHostsUnHealthy  string = "aws.elb.hosts.unhealthy"
	awsEC2CPUDesc         string = "The average CPU Utilization, gathered at a 60 second interval and averaged over five minutes."
	awsEC2NetBytesDesc    string = "The average bytes transmitted or received via network, gathered at a 60 second interval and averaged over five minutes."
	awsEC2DiskBytesDesc   string = "The average bytes written or read via disk, gathered at a 60 second interval and averaged over five minutes."
	awsEC2DiskOpsDesc     string = "The average disk operations, either written or read, gathered at a 60 second interval and averaged over five minutes."
	awsEC2StatusCheckDesc string = "The EC2 Status Check, which includes both instance-level and system-level drill-down, gathered every 60 seconds."
	awsELBLatencyDesc     string = "The minimum, maximum and average latency as reported by the load balancer, gathered at a 60 second interval and averaged over five minutes."
	awsELBHostCountDesc   string = "The number of instances in what the Elastic Load Balancer considers a healthy state, gathered every 60 seconds."
)

func AWS(accessKey, secretKey, region string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_aws(accessKey, secretKey, region)
		},
		Interval: 60 * time.Second,
		name:     fmt.Sprintf("aws-%s", region),
	})
}

func c_aws(accessKey, secretKey, region string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	creds := aws.Creds(accessKey, secretKey, "")
	if creds == nil {
		return nil, fmt.Errorf("Unable to make creds")
	}
	ecc := ec2.New(creds, region, nil)
	if ecc == nil {
		return nil, fmt.Errorf("Unable to login to EC2")
	}
	elb := elb.New(creds, region, nil)
	if elb == nil {
		return nil, fmt.Errorf("Unable to login to ELB")
	}
	cw := cloudwatch.New(creds, region, nil)
	if cw == nil {
		return nil, fmt.Errorf("Unable to login to CloudWatch")
	}
	instances, err := AWSGetInstances(*ecc)
	if err != nil {
		slog.Info("No EC2 Instances found.")
	}
	loadbalancers, err := AWSGetLoadBalancers(*elb)
	if err != nil {
		slog.Info("No ELB Load Balancecrs found.")
	}
	for _, loadbalancer := range loadbalancers {
		AWSGetELBLatency(*cw, &md, loadbalancer)
		AWSGetELBHostCounts(*cw, &md, loadbalancer)
	}
	for _, instance := range instances {
		AWSGetCPU(*cw, &md, instance)
		AWSGetNetwork(*cw, &md, instance)
		AWSGetDiskBytes(*cw, &md, instance)
		AWSGetDiskOps(*cw, &md, instance)
		AWSGetStatusChecks(*cw, &md, instance)
	}
	return md, nil
}

func AWSGetInstances(ecc ec2.EC2) ([]ec2.Instance, error) {
	instancelist := []ec2.Instance{}
	resp, err := ecc.DescribeInstances(nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to describe EC2 Instances")
	}
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			instancelist = append(instancelist, instance)
		}
	}
	return instancelist, nil
}

func AWSGetLoadBalancers(lb elb.ELB) ([]elb.LoadBalancerDescription, error) {
	lblist := []elb.LoadBalancerDescription{}
	resp, err := lb.DescribeLoadBalancers(nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to describe ELB Balancers")
	}
	for _, loadbalancer := range resp.LoadBalancerDescriptions {
		lblist = append(lblist, loadbalancer)
	}
	return lblist, nil
}

func AWSGetCPU(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -600),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("CPUUtilization"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Percent"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	tags := opentsdb.TagSet{
		"instance": *instance.InstanceID,
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsCPU, datapoint.Timestamp.Unix(), *datapoint.Average, tags, metadata.Gauge, metadata.Pct, awsEC2CPUDesc)
	}
	return nil
}
func AWSGetNetwork(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -600),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("NetworkIn"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Bytes"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsNetwork, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "direction": "in"}, metadata.Gauge, metadata.Bytes, awsEC2NetBytesDesc)
	}
	search.MetricName = aws.String("NetworkOut")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsNetwork, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "direction": "out"}, metadata.Gauge, metadata.Bytes, awsEC2NetBytesDesc)
	}
	return nil
}

func AWSGetDiskBytes(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -600),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("DiskReadBytes"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Bytes"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskBytes, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "operation": "read"}, metadata.Gauge, metadata.Bytes, awsEC2DiskBytesDesc)
	}
	search.MetricName = aws.String("DiskWriteBytes")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskBytes, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "operation": "write"}, metadata.Gauge, metadata.Bytes, awsEC2DiskBytesDesc)
	}
	return nil
}

func AWSGetDiskOps(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -600),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("DiskReadOps"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Count"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskOps, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "operation": "read"}, metadata.Gauge, metadata.Count, awsEC2DiskOpsDesc)
	}
	search.MetricName = aws.String("DiskWriteOps")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsEC2DiskOps, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "operation": "write"}, metadata.Gauge, metadata.Count, awsEC2DiskOpsDesc)
	}
	return nil
}

func AWSGetStatusChecks(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, instance ec2.Instance) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -60),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("StatusCheckFailed"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/EC2"),
		Unit:       aws.String("Count"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("InstanceId"), Value: instance.InstanceID}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID}, metadata.Gauge, metadata.Count, awsEC2StatusCheckDesc)
	}
	search.MetricName = aws.String("StatusCheckFailed_Instance")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "category": "instance"}, metadata.Gauge, metadata.Count, awsEC2StatusCheckDesc)
	}
	search.MetricName = aws.String("StatusCheckFailed_System")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsStatusCheckFailed, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"instance": *instance.InstanceID, "category": "system"}, metadata.Gauge, metadata.Count, awsEC2StatusCheckDesc)
	}
	return nil
}

func AWSGetELBLatency(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, loadbalancer elb.LoadBalancerDescription) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -4000),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("Latency"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average", "Minimum", "Maximum"},
		Namespace:  aws.String("AWS/ELB"),
		Unit:       aws.String("Seconds"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("LoadBalancerName"), Value: loadbalancer.LoadBalancerName}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsELBLatencyMin, datapoint.Timestamp.Unix(), *datapoint.Minimum, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, awsELBLatencyDesc)
		AddTS(md, awsELBLatencyMax, datapoint.Timestamp.Unix(), *datapoint.Maximum, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, awsELBLatencyDesc)
		AddTS(md, awsELBLatencyAvg, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Second, awsELBLatencyDesc)
	}
	return nil
}
func AWSGetELBHostCounts(cw cloudwatch.CloudWatch, md *opentsdb.MultiDataPoint, loadbalancer elb.LoadBalancerDescription) error {
	search := cloudwatch.GetMetricStatisticsInput{
		StartTime:  time.Now().UTC().Add(time.Second * -60),
		EndTime:    time.Now().UTC(),
		MetricName: aws.String("HealthyHostCount"),
		Period:     aws.Integer(60),
		Statistics: []string{"Average"},
		Namespace:  aws.String("AWS/ELB"),
		Unit:       aws.String("Count"),
		Dimensions: []cloudwatch.Dimension{{Name: aws.String("LoadBalancerName"), Value: loadbalancer.LoadBalancerName}},
	}
	resp, err := cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	for _, datapoint := range resp.Datapoints {
		AddTS(md, awsELBHostsHealthy, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, awsELBHostCountDesc)
	}
	search.MetricName = aws.String("UnhealthyHostCount")
	resp, err = cw.GetMetricStatistics(&search)
	if err != nil {
		return fmt.Errorf("Error getting Metric Statistics: %s", err)
	}
	if resp.Datapoints == nil {
		AddTS(md, awsELBHostsUnHealthy, time.Now().UTC().Unix(), 0, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, awsELBHostCountDesc)
	} else {

		for _, datapoint := range resp.Datapoints {
			AddTS(md, awsELBHostsUnHealthy, datapoint.Timestamp.Unix(), *datapoint.Average, opentsdb.TagSet{"loadbalancer": *loadbalancer.LoadBalancerName}, metadata.Gauge, metadata.Count, awsELBHostCountDesc)
		}
	}
	return nil
}
