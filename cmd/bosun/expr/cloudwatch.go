package expr

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"bosun.org/cloudwatch"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
)

// cloudwatch defines functions for use with amazon cloudwatch api
var CloudWatch = map[string]parse.Func{

	"cw": {
		Args: []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString,
			models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          cloudwatchTagQuery,
		F:             CloudWatchQuery,
		PrefixEnabled: true,
	},
}

var PeriodParseError = errors.New("Could not parse the period value")
var StartParseError = errors.New("Could not parse the start value")
var EndParseError = errors.New("Could not parse the end value")
var DimensionParseError = errors.New("dimensions must be in format key:value")

var isNumber = regexp.MustCompile("^\\d+$")

func parseCloudWatchResponse(req *cloudwatch.Request, s *cloudwatch.Response, multiRegion bool) ([]*Result, error) {
	const parseErrFmt = "cloudwatch ParseError (%s): %s"
	var dps Series
	if s == nil {
		return nil, fmt.Errorf(parseErrFmt, req.Metric, "empty response")
	}
	results := make([]*Result, 0)

	for _, result := range s.Raw.MetricDataResults {
		if len(result.Timestamps) == 0 {
			continue
		}
		tags := make(opentsdb.TagSet)
		for k, v := range s.TagSet[*result.Id] {
			tags[k] = v
		}
		dps = make(Series)
		for x, t := range result.Timestamps {
			dps[*t] = *result.Values[x]
		}

		r := Result{
			Value: dps,
			Group: tags,
		}

		if multiRegion {
			r.Group["bosun-region"] = req.Region
		}

		results = append(results, &r)
	}

	return results, nil
}

func hasWildcardDimension(dimensions string) bool {
	return strings.Contains(dimensions, "*")
}

func parseDimensions(dimensions string) ([][]cloudwatch.Dimension, error) {
	dl := make([][]cloudwatch.Dimension, 0)
	if len(strings.TrimSpace(dimensions)) == 0 {
		return dl, nil
	}
	dims := strings.Split(dimensions, ",")

	l := make([]cloudwatch.Dimension, 0)
	for _, row := range dims {
		dim := strings.Split(row, ":")
		if len(dim) != 2 {
			return nil, DimensionParseError
		}
		l = append(l, cloudwatch.Dimension{Name: dim[0], Value: dim[1]})
	}
	dl = append(dl, l)

	return dl, nil
}

func parseDurations(s, e, p string) (start, end, period opentsdb.Duration, err error) {

	start, err = opentsdb.ParseDuration(s)
	if err != nil {
		return start, end, period, StartParseError
	}
	end = opentsdb.Duration(0)
	if e != "" {
		end, err = opentsdb.ParseDuration(e)
		if err != nil {
			return start, end, period, EndParseError
		}
	}

	// to maintain backwards compatibility assume that period without time unit is seconds
	if isNumber.MatchString(p) {
		p += "s"
	}
	period, err = opentsdb.ParseDuration(p)
	if err != nil {
		return start, end, period, PeriodParseError
	}
	return
}

func CloudWatchQuery(prefix string, e *State, region, namespace, metric, period, statistic, dimensions, sduration, eduration string) (*Results, error) {

	r := new(Results)

	regions := strings.Split(region, ",")
	if len(regions) == 0 {
		return r, nil
	}

	var wg sync.WaitGroup
	queryResults := []*Results{}

	// reqCh (Request Channel) is populated with cloudwatch requests for each region
	reqCh := make(chan cloudwatch.Request, len(regions))
	// resCh (Result Channel) contains the timeseries responses for requests for region
	resCh := make(chan *Results, len(regions))
	// errCh (Error Channel) contains any request errors
	errCh := make(chan error, len(regions))

	// a worker makes a getMetricData request for a region
	worker := func() {
		for req := range reqCh {
			res := []*Result{}
			data, err := getCloudwatchData(e, &req)
			if err == nil {
				res, err = parseCloudWatchResponse(&req, &data, len(regions) > 1)
				resCh <- &Results{Results: res}
			}
			errCh <- err
		}
		defer wg.Done()
	}

	// Create N workers to parallelize multiple requests at once since each region requires an HTTP request
	for i := 0; i < e.CloudWatchContext.GetConcurrency(); i++ {
		wg.Add(1)
		go worker()
	}

	sd, ed, p, err := parseDurations(sduration, eduration, period)
	if err != nil {
		return r, err
	}

	timingString := fmt.Sprintf(`querying %d regions for metric:"%v"`, len(regions), metric)
	e.Timer.StepCustomTiming("cloudwatch", "query", timingString, func() {
		// Feed region queries into the request channel which the workers will consume

		for _, r := range regions {

			// The times are rounded to a whole period. This improves
			// both the caching of the query results as well as the query performance for
			// the reasons outlined in the aws sdk docs here
			// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html

			// round down start time
			st := e.now.Add(-time.Duration(sd)).Truncate(time.Duration(p))
			// round up end time
			et := e.now.Add(-time.Duration(ed)).Truncate(time.Duration(p)).Add(time.Duration(p))

			req := cloudwatch.Request{
				Start:           &st,
				End:             &et,
				Region:          r,
				Namespace:       namespace,
				Metric:          metric,
				Period:          int64(p.Seconds()),
				Statistic:       statistic,
				DimensionString: dimensions,
				Profile:         prefix,
			}
			reqCh <- req
		}
		close(reqCh)
		wg.Wait() // Wait for all the workers to finish
	})
	close(resCh)
	close(errCh)

	// Gather errors from the request and return an error if any of the requests failled
	errs := []string{}
	for err := range errCh {
		if err == nil {
			continue
		}
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return r, fmt.Errorf(strings.Join(errs, " :: "))
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

func getCloudwatchData(e *State, req *cloudwatch.Request) (resp cloudwatch.Response, err error) {
	e.cloudwatchQueries = append(e.cloudwatchQueries, *req)

	key := req.CacheKey()
	getFn := func() (interface{}, error) {

		d, err := parseDimensions(req.DimensionString)

		if hasWildcardDimension(req.DimensionString) {
			lr := cloudwatch.LookupRequest{
				Region:     req.Region,
				Namespace:  req.Namespace,
				Metric:     req.Metric,
				Dimensions: d,
				Profile:    req.Profile,
			}
			d, err = e.CloudWatchContext.LookupDimensions(&lr)
			if err != nil {
				return resp, err
			}
			if len(d) == 0 {
				return resp, fmt.Errorf("Wildcard dimensionString did not match any cloudwatch metrics in region %s", req.Region)
			}
		}
		req.Dimensions = d
		return e.CloudWatchContext.Query(req)
	}

	var val interface{}
	var hit bool
	val, err, hit = e.Cache.Get(key, getFn)

	collectCacheHit(e.Cache, "cloudwatch", hit)
	resp = val.(cloudwatch.Response)

	return
}

func cloudwatchTagQuery(args []parse.Node) (parse.Tags, error) {
	t := make(parse.Tags)
	n := args[5].(*parse.StringNode)
	for _, s := range strings.Split(n.Text, ",") {
		if s != "" {
			g := strings.Split(s, ":")
			if g[0] != "" {
				t[g[0]] = struct{}{}
			}
		}
	}
	return t, nil
}
