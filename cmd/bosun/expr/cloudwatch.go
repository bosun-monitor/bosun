package expr

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"strings"
	"time"

	"bosun.org/cloudwatch"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"sync"
)

// cloudwatch defines functions for use with amazon cloudwatch api
var CloudWatch = map[string]parse.Func{
	"cwdimq": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeCWDimensionList,
		F:             CloudWatchQueryDimensions,
		PrefixEnabled: true,
	},
	"cwdim": {
		Args:          []models.FuncType{models.TypeString},
		Return:        models.TypeCWDimensionList,
		F:             CloudWatchBuildDimensions,
		PrefixEnabled: true,
	},
	"cw": {
		Args: []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeScalar,
			models.TypeString, models.TypeCWDimensionList, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          cloudwatchTagQuery,
		F:             CloudWatchQuery,
		PrefixEnabled: true,
	},
}

/*

Dimension list specified as a array of json objects which map from an attribute to a value
Example 1, one attribute with multiple values, which should return 3 series.

[
 { "InstanceId":"i-12345" },
 { "InstanceId":"i-56789" },
 { "InstanceId":"i-abcdef" }
]

Example 2:
S3 metrics require two dimensions. This will return two series.
[
{ "BucketName": "WebStaticContent", "StorageType": "Regular" },
{ "BucketName": "Backups", "StorageType": "InfrequentAccess" }
]

*/

// parse json representation into dimension list.
func parseDimensionList(dimensions string) ([][]cloudwatch.Dimension, error) {

	var raw []*json.RawMessage
	var ds []cloudwatch.Dimension
	var dimension map[string]string

	dl := make([][]cloudwatch.Dimension, 0)
	err := json.Unmarshal([]byte(dimensions), &raw)

	if err != nil {
		return dl, fmt.Errorf("invalid dimension list json %s ", err)
	}
	for _, v := range raw {
		err = json.Unmarshal(*v, &dimension)
		if err != nil {
			return dl, fmt.Errorf("invalid dimension list json %s ", err)
		}
		ds = make([]cloudwatch.Dimension, 0)

		for k, v := range dimension {
			d := cloudwatch.Dimension{
				Name:  k,
				Value: v,
			}
			ds = append(ds, d)
		}

		dl = append(dl, ds)

	}

	return dl, nil
}

// parse json object into list of ec2_describe filter
func parseFilters(f string) ([]*ec2.Filter, error) {

	var filter map[string][]*string
	var raw []*json.RawMessage
	filters := []*ec2.Filter{}

	err := json.Unmarshal([]byte(f), &raw)
	if err != nil {
		return filters, fmt.Errorf("invalid filter json %s", err)
	}
	for _, v := range raw {
		err := json.Unmarshal(*v, &filter)
		if err != nil {
			return filters, fmt.Errorf("invalid filter json %s ", err)
		}
		for i, j := range filter {
			f := ec2.Filter{
				Name:   aws.String(i),
				Values: j,
			}
			filters = append(filters, &f)
		}

	}
	return filters, err
}

func CloudWatchQuery(prefix string, e *State, region, namespace, metric string, period float64, statistic string,
	queries cloudwatch.DimensionList, sduration, eduration string) (r *Results, err error) {
	concurrency := e.Backends.CloudWatchContext.GetConcurrency()
	r = new(Results)

	nQueries := len(queries.Groups)
	if nQueries == 0 {
		return r, nil
	}
	queryResults := []*Results{}
	var wg sync.WaitGroup
	// reqCh (Request Channel) is populated with Cloudwatch requests, and requests are pulled from channel to make
	// a time series request per dimension set
	reqCh := make(chan cloudwatch.Request, nQueries)
	// resCh (Result Channel) contains the timeseries responses for requests for resource
	resCh := make(chan *Results, nQueries)
	// errCh (Error Channel) contains any request errors
	errCh := make(chan error, nQueries)
	// a worker makes a time series request for a resource
	worker := func() {
		for req := range reqCh {
			results, err := cloudwatchRequest(e, &req)
			if err == nil {
				res := new(Results)
				res.Results = results
				resCh <- res
			}
			errCh <- err
		}
		defer wg.Done()
	}
	// Create N workers to parallelize multiple requests at once since he resource requires an HTTP request
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	sd, err := opentsdb.ParseDuration(sduration)
	if err != nil {
		return nil, err
	}
	ed := opentsdb.Duration(0)
	if eduration != "" {
		ed, err = opentsdb.ParseDuration(eduration)
		if err != nil {
			return nil, err
		}
	}
	st := e.now.Add(-time.Duration(sd))
	et := e.now.Add(-time.Duration(ed))

	timingString := fmt.Sprintf(`%v queries for metric:"%v" using client "%v"`, 1, metric, prefix)
	e.Timer.StepCustomTiming("cloudwatch", "query-multi", timingString, func() {
		// Feed resources into the request channel which the workers will consume

		for _, q := range queries.Groups {

			req := cloudwatch.Request{
				Start:      &st,
				End:        &et,
				Region:     region,
				Namespace:  namespace,
				Metric:     metric,
				Period:     int64(period),
				Statistic:  statistic,
				Dimensions: q,
				Profile:    prefix,
			}
			reqCh <- req
		}

		close(reqCh)
		wg.Wait() // Wait for all the workers to finish
	})
	close(resCh)
	close(errCh)

	// Gather errors from the request and return an error if any of the requests failled
	errors := []string{}
	for err := range errCh {
		if err == nil {
			continue
		}
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		return r, fmt.Errorf(strings.Join(errors, " :: "))
	}
	// Gather all the query results
	for res := range resCh {
		queryResults = append(queryResults, res)
	}
	if len(queryResults) == 1 { // no need to merge if there is only one item
		return queryResults[0], nil
	}
	// Merge the query results into a single seriesSet
	r, err = Merge(e, queryResults...)

	return r, err

}

func CloudWatchQueryDimensions(prefix string, e *State, region, family, attribute, filter string) (r *Results, err error) {

	var dl *cloudwatch.DimensionList
	r = new(Results)

	f, err := parseFilters(filter)
	if err != nil {
		return nil, err
	}
	switch family {
	case "EC2":
		dl, err = e.CloudWatchContext.Describe(region, prefix, attribute, f)
	default:
		return nil, fmt.Errorf("invalid family string: %s", family)
	}

	if err == nil {
		r.Results = append(r.Results, &Result{Value: dl})
	}
	return
}

func CloudWatchBuildDimensions(prefix string, e *State, dimensions string) (r *Results, err error) {
	dl := cloudwatch.DimensionList{}
	r = new(Results)
	groups, err := parseDimensionList(dimensions)

	if err == nil {
		dl.Groups = groups
		r.Results = append(r.Results, &Result{Value: dl})
	}
	return
}

func cloudwatchRequest(e *State, req *cloudwatch.Request) (result []*Result, err error) {

	e.cloudwatchQueries = append(e.cloudwatchQueries, *req)
	key := req.CacheKey()
	getFn := func() (interface{}, error) {

		b, _ := json.MarshalIndent(req, "", "  ")
		var resp cloudwatch.Response

		e.Timer.StepCustomTiming("cloudwatch", "query", string(b), func() {
			resp, err = e.CloudWatchContext.Query(req)
		})
		return resp, err
	}

	var val interface{}
	var hit bool
	val, err, hit = e.Cache.Get(key, getFn)
	collectCacheHit(e.Cache, "cloudwatch", hit)
	resp := val.(cloudwatch.Response)

	if &resp == nil {
		return nil, fmt.Errorf("cloudwatch ParseError (%s): %s", req.Metric, "empty response")
	}
	results := make([]*Result, 0)
	dps := make(Series)
	tags := make(opentsdb.TagSet)
	for _, d := range req.Dimensions {
		tags[d.Name] = d.Value
	}

	switch req.Statistic {
	case "Sum":
		for _, x := range resp.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Sum
		}
	case "Average":
		for _, x := range resp.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Average
		}
	case "Minimum":
		for _, x := range resp.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Minimum
		}
	case "Maximum":
		for _, x := range resp.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Maximum
		}
	case "SampleCount":
		for _, x := range resp.Raw.Datapoints {
			dps[*x.Timestamp] = *x.SampleCount
		}
	default:
		return nil, fmt.Errorf("no such statistic '%s'", req.Statistic)
	}

	results = append(results, &Result{
		Value: dps,
		Group: tags,
	})
	return results, nil
}

func cloudwatchTagQuery(args []parse.Node) (parse.Tags, error) {
	var f *parse.FuncNode
	t := make(parse.Tags)

	switch args[5].(type) {
	case *parse.PrefixNode:
		p := args[5].(*parse.PrefixNode)
		f = p.Arg.(*parse.FuncNode)

	case *parse.FuncNode:
		f = args[5].(*parse.FuncNode)

	default:
		return nil, fmt.Errorf("Unexpected type")
	}

	if f.Name == "cwdimq" {
		name := f.Args[2].(*parse.StringNode).Text
		t[name] = struct{}{}
	}
	if f.Name == "cwdim" {
		dimensionString := f.Args[0].(*parse.StringNode).Text
		d, err := parseDimensionList(dimensionString)
		if err == nil {
			for _, i := range d {
				for _, j := range i {
					t[j.Name] = struct{}{}
				}
			}
		}
	}

	return t, nil
}
