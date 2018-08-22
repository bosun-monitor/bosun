package expr

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bosun.org/cloudwatch"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
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

func parseCloudWatchResponse(req *cloudwatch.Request, s *cloudwatch.Response) ([]*Result, error) {
	const parseErrFmt = "cloudwatch ParseError (%s): %s"
	if s == nil {
		return nil, fmt.Errorf(parseErrFmt, req.Metric, "empty response")
	}
	results := make([]*Result, 0)
	dps := make(Series)
	tags := make(opentsdb.TagSet)
	for _, d := range req.Dimensions {
		tags[d.Name] = d.Value
	}

	switch req.Statistic {
	case "Sum":
		for _, x := range s.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Sum
		}
	case "Average":
		for _, x := range s.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Average
		}
	case "Minimum":
		for _, x := range s.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Minimum
		}
	case "Maximum":
		for _, x := range s.Raw.Datapoints {
			dps[*x.Timestamp] = *x.Maximum
		}
	default:
		return nil, fmt.Errorf("No such statistic '%s'", req.Statistic)
	}

	results = append(results, &Result{
		Value: dps,
		Group: tags,
	})
	return results, nil
}

func parseDimensions(dimensions string) []cloudwatch.Dimension {
	parsed := make([]cloudwatch.Dimension, 0)
	dims := strings.Split(dimensions, ",")
	for _, row := range dims {
		dim := strings.Split(row, ":")
		parsed = append(parsed, cloudwatch.Dimension{Name: dim[0], Value: dim[1]})
	}
	return parsed
}

func CloudWatchQuery(prefix string, e *State, T miniprofiler.Timer, region, namespace, metric, period, statistic, dimensions, sduration, eduration string) (*Results, error) {
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
	d := parseDimensions(dimensions)
	st := e.now.Add(-time.Duration(sd))
	et := e.now.Add(-time.Duration(ed))
	req := &cloudwatch.Request{
		Start:      &st,
		End:        &et,
		Region:     region,
		Namespace:  namespace,
		Metric:     metric,
		Period:     period,
		Statistic:  statistic,
		Dimensions: d,
		Profile:    prefix,
	}
	s, err := timeCloudwatchRequest(e, T, req)
	if err != nil {
		return nil, err
	}
	r := new(Results)
	results, err := parseCloudWatchResponse(req, &s)
	if err != nil {
		return nil, err
	}
	r.Results = results
	return r, nil
}

func timeCloudwatchRequest(e *State, T miniprofiler.Timer, req *cloudwatch.Request) (resp cloudwatch.Response, err error) {
	e.cloudwatchQueries = append(e.cloudwatchQueries, *req)
	b, _ := json.MarshalIndent(req, "", "  ")
	T.StepCustomTiming("cloudwatch", "query", string(b), func() {
		key := req.CacheKey()

		getFn := func() (interface{}, error) {
			return e.CloudWatchContext.Query(req)
		}
		var val interface{}
		var hit bool
		val, err, hit = e.Cache.Get(key, getFn)
		collectCacheHit(e.Cache, "cloudwatch", hit)
		resp = val.(cloudwatch.Response)

	})
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
