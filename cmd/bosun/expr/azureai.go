package expr

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/opentsdb"
	ainsights "github.com/Azure/azure-sdk-for-go/services/appinsights/v1/insights"
	"github.com/kylebrandt/boolq"
)

// AzureAIQuery queries the Azure Application Insights API for metrics data and transforms the response into a series set
func AzureAIQuery(prefix string, e *State, metric, segmentCSV, filter string, apps AzureApplicationInsightsApps, agtype, interval, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	if apps.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use apps from client "%v" on a query with client "%v"`, apps.Prefix, prefix)
	}
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.AIMetricsClient

	// Parse Relative Time to absolute time
	timespan, err := azureTimeSpan(e, sdur, edur)
	if err != nil {
		return nil, err
	}

	// Handle the timegrain (downsampling)
	var tg string
	if interval != "" {
		tg = *azureIntervalToTimegrain(interval)
	} else {
		tg = "PT1M"
	}

	// The SDK Get call requires that segments/dimensions be of type MetricsSegment
	segments := []ainsights.MetricsSegment{}
	hasSegments := segmentCSV != ""
	if hasSegments {
		for _, s := range strings.Split(segmentCSV, ",") {
			segments = append(segments, ainsights.MetricsSegment(s))
		}
	}
	segLen := len(segments)

	// The SDK Get call required that that the aggregation be of type MetricsAggregation
	agg := []ainsights.MetricsAggregation{ainsights.MetricsAggregation(agtype)}

	// Since the response is effectively grouped by time, and our series set is grouped by tags, this stores
	// TagKey -> to series map
	seriesMap := make(map[string]Series)

	// Main Loop - With segments/dimensions values will be nested, otherwise values are in the root
	for _, app := range apps.Applications {
		appName, err := opentsdb.Clean(app.ApplicationName)
		if err != nil {
			return r, err
		}
		cacheKey := strings.Join([]string{prefix, app.AppId, metric, timespan, tg, agtype, segmentCSV, filter}, ":")
		// Each request (per application) is cached
		getFn := func() (interface{}, error) {
			req, err := c.GetPreparer(context.Background(), app.AppId, ainsights.MetricID(metric), timespan, &tg, agg, segments, nil, "", filter)
			if err != nil {
				return nil, err
			}
			var resp ainsights.MetricsResult
			e.Timer.StepCustomTiming("azureai", "query", req.URL.String(), func() {
				hr, sendErr := c.GetSender(req)
				if sendErr == nil {
					resp, err = c.GetResponder(hr)
				} else {
					err = sendErr
				}
			})
			return resp, err
		}
		val, err, hit := e.Cache.Get(cacheKey, getFn)
		if err != nil {
			return r, err
		}
		collectCacheHit(e.Cache, "azureai_ts", hit)
		res := val.(ainsights.MetricsResult)

		basetags := opentsdb.TagSet{"app": appName}

		for _, seg := range *res.Value.Segments {
			handleInnerSegment := func(s ainsights.MetricsSegmentInfo) error {
				met, ok := s.AdditionalProperties[metric]
				if !ok {
					return fmt.Errorf("expected additional properties not found on inner segment while handling azure query")
				}
				metMap, ok := met.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unexpected type for additional properties not found on inner segment while handling azure query")
				}
				metVal, ok := metMap[agtype]
				if !ok {
					return fmt.Errorf("expected aggregation value for aggregation %v not found on inner segment while handling azure query", agtype)
				}
				tags := opentsdb.TagSet{}
				if hasSegments {
					key := string(segments[segLen-1])
					val, ok := s.AdditionalProperties[key]
					if !ok {
						return fmt.Errorf("unexpected dimension/segment key %v not found in response", key)
					}
					sVal, ok := val.(string)
					if !ok {
						return fmt.Errorf("unexpected dimension/segment value for key %v in response", key)
					}
					tags[key] = sVal
				}
				tags = tags.Merge(basetags)
				err := tags.Clean()
				if err != nil {
					return err
				}
				if _, ok := seriesMap[tags.Tags()]; !ok {
					seriesMap[tags.Tags()] = make(Series)
				}
				if v, ok := metVal.(float64); ok && seg.Start != nil {
					seriesMap[tags.Tags()][seg.Start.Time] = v
				}
				return nil
			}

			// Simple case with no Segments/Dimensions
			if !hasSegments {
				err := handleInnerSegment(seg)
				if err != nil {
					return r, err
				}
				continue
			}

			// Case with Segments/Dimensions
			next := &seg
			// decend (fast forward) to the next nested MetricsSegmentInfo by moving the 'next' pointer
			decend := func(dim string) error {
				if next == nil || next.Segments == nil || len(*next.Segments) == 0 {
					return fmt.Errorf("unexpected insights response while handling dimension %s", dim)
				}
				next = &(*next.Segments)[0]
				return nil
			}
			if segLen > 1 {
				if err := decend("root-level"); err != nil {
					return r, err
				}
			}
			// When multiple dimensions are requests, there are nested MetricsSegmentInfo objects
			// The higher levels just contain all the dimension key-value pairs except the last.
			// So we fast forward to the depth that has the last tag pair and the metric values
			// collect tags along the way
			for i := 0; i < segLen-1; i++ {
				segStr := string(segments[i])
				basetags[segStr] = next.AdditionalProperties[segStr].(string)
				if i != segLen-2 { // the last dimension/segment will be in same []MetricsSegmentInfo slice as the metric value
					if err := decend(string(segments[i])); err != nil {
						return r, err
					}
				}
			}
			if next == nil {
				return r, fmt.Errorf("unexpected segement/dimension in insights response")
			}
			for _, innerSeg := range *next.Segments {
				err := handleInnerSegment(innerSeg)
				if err != nil {
					return r, err
				}
			}
		}
	}

	// Transform seriesMap into seriesSet (ResultSlice)
	for k, series := range seriesMap {
		tags, err := opentsdb.ParseTags(k)
		if err != nil {
			return r, err
		}
		r.Results = append(r.Results, &Result{
			Value: series,
			Group: tags,
		})
	}
	return r, nil
}

// AzureApplicationInsightsApp in collection of properties for each Azure Application Insights Resource
type AzureApplicationInsightsApp struct {
	ApplicationName string
	AppId           string
	Tags            map[string]string
}

// AzureApplicationInsightsApps is a container for a list of AzureApplicationInsightsApp objects
// It is a bosun type since it passed to Azure Insights query functions
type AzureApplicationInsightsApps struct {
	Applications []AzureApplicationInsightsApp
	Prefix       string
}

// AzureAIFilterApps filters a list of applications based on the name of the app, or the Azure tags associated with the application resource
func AzureAIFilterApps(prefix string, e *State, apps AzureApplicationInsightsApps, filter string) (r *Results, err error) {
	r = new(Results)
	// Parse the filter once and then apply it to each item in the loop
	bqf, err := boolq.Parse(filter)
	if err != nil {
		return r, err
	}
	filteredApps := AzureApplicationInsightsApps{Prefix: apps.Prefix}
	for _, app := range apps.Applications {
		match, err := boolq.AskParsedExpr(bqf, app)
		if err != nil {
			return r, err
		}
		if match {
			filteredApps.Applications = append(filteredApps.Applications, app)
		}
	}
	r.Results = append(r.Results, &Result{Value: filteredApps})
	return
}

// Ask makes an AzureApplicationInsightsApp a github.com/kylebrandt/boolq Asker, which allows it to
// to take boolean expressions to create true/false conditions for filtering
func (app AzureApplicationInsightsApp) Ask(filter string) (bool, error) {
	sp := strings.SplitN(filter, ":", 2)
	if len(sp) != 2 {
		return false, fmt.Errorf("bad filter, filter must be in k:v format, got %v", filter)
	}
	key := strings.ToLower(sp[0]) // Make key case insensitive
	value := sp[1]
	switch key {
	case azureTagName:
		re, err := regexp.Compile(value)
		if err != nil {
			return false, err
		}
		if re.MatchString(app.ApplicationName) {
			return true, nil
		}
	default:
		if tagV, ok := app.Tags[key]; ok {
			re, err := regexp.Compile(value)
			if err != nil {
				return false, err
			}
			if re.MatchString(tagV) {
				return true, nil
			}
		}

	}
	return false, nil
}

// AzureAIListApps get a list of all applications on the subscription and returns those apps in a AzureApplicationInsightsApps within the result
func AzureAIListApps(prefix string, e *State) (r *Results, err error) {
	r = new(Results)
	// Verify prefix is a defined resource and fetch the collection of clients
	key := fmt.Sprintf("AzureAIAppCache:%s:%s", prefix, time.Now().Truncate(time.Minute*1)) // https://github.com/golang/groupcache/issues/92

	getFn := func() (interface{}, error) {
		cc, clientFound := e.Backends.AzureMonitor[prefix]
		if !clientFound {
			return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
		}
		c := cc.AIComponentsClient
		applist := AzureApplicationInsightsApps{Prefix: prefix}
		for rList, err := c.ListComplete(context.Background()); rList.NotDone(); err = rList.Next() {
			if err != nil {
				return r, err
			}
			comp := rList.Value()
			azTags := make(map[string]string)
			if comp.Tags != nil {
				for k, v := range comp.Tags {
					if v != nil {
						azTags[k] = *v
						continue
					}
					azTags[k] = ""
				}
			}
			if comp.ID != nil && comp.ApplicationInsightsComponentProperties != nil && comp.ApplicationInsightsComponentProperties.AppID != nil {
				applist.Applications = append(applist.Applications, AzureApplicationInsightsApp{
					ApplicationName: *comp.Name,
					AppId:           *comp.ApplicationInsightsComponentProperties.AppID,
					Tags:            azTags,
				})
			}
		}
		r.Results = append(r.Results, &Result{Value: applist})
		return r, nil
	}
	val, err, hit := e.Cache.Get(key, getFn)
	collectCacheHit(e.Cache, "azure_aiapplist", hit)
	if err != nil {
		return r, err
	}
	return val.(*Results), nil
}

// AzureAIMetricMD returns metric metadata for the listed AzureApplicationInsightsApps. This is not meant
// as core expression function, but rather one for interactive inspection through the expression UI.
func AzureAIMetricMD(prefix string, e *State, apps AzureApplicationInsightsApps) (r *Results, err error) {
	r = new(Results)
	if apps.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use apps from client "%v" on a query with client "%v"`, apps.Prefix, prefix)
	}
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.AIMetricsClient
	for _, app := range apps.Applications {
		md, err := c.GetMetadata(context.Background(), app.AppId)
		if err != nil {
			return r, err
		}
		r.Results = append(r.Results, &Result{
			Value: Info{md.Value},
			Group: opentsdb.TagSet{"app": app.ApplicationName},
		})
	}
	return
}

// azAITags is the tag function for the "az" expression function
func azAITags(args []parse.Node) (parse.Tags, error) {
	tags := parse.Tags{"app": struct{}{}}
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	if len(csvTags) == 1 && csvTags[0] == "" {
		return tags, nil
	}
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}
