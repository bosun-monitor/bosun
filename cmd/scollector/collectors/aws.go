package collectors

import (
    "bytes"
    "encoding/xml"
    "fmt"
    "io"
    "strconv"

    //"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"

    "github.com/awslabs/aws-sdk-go/aws"
    "github.com/awslabs/aws-sdk-go/gen/ec2"
    //"github.com/awslabs/aws-sdk-go/gen/elb"
    "github.com/awslabs/aws-sdk-go/gen/cloudwatch"

)

// Vsphere registers a vSphere collector.
func AWS(accessKey, secretKey, region string) {
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_aws(accessKey, secretKey, region)
		},
		name: fmt.Sprintf("aws-%s", region),
	})
}

func c_aws(accessKey, secretKey, region string) (opentsdb.MultiDataPoint, error) {
    creds := aws.Creds(accessKey, secretKey, "")
    ecc := ec2.New(creds, region, nil)
    //elb := elb.New(creds, region, nil)
    cw := cloudwatch.New(creds, region, nil)

    instances, err := AWSGetInstances(&ecc)
    if err != nil {
        // Do something useful in error
    }
	return md, nil

        //Add(md, osDiskTotal, i, tags, metadata.Gauge, metadata.Bytes, "")

}

func AWSGetInstances(ecc ec2.ec2) []Instance{
    resp, err := ecc.DescribeInstances(nil)
    if err != nil {
        return nil, 1
    }
    instancelist := []Instance{}

    for _, instance := range resp.Reservations {
        append(instancelist, instance)
    }
    return instancelist, nil
}
