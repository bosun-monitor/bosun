package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/cmd/bosun/expr/tsdbs"
	"bosun.org/opentsdb"
	ainsights "github.com/Azure/azure-sdk-for-go/services/appinsights/v1/insights"
)

// AIQuery queries the Azure Application Insights API for metrics data and transforms the response into a series set
func AIQuery(prefix string, e *expr.State, metric, segmentCSV, filter string, apps tsdbs.AzureApplicationInsightsApps, agtype, interval, sdur, edur string) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
	if apps.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use apps from client "%v" on a query with client "%v"`, apps.Prefix, prefix)
	}
	cc, clientFound := e.TSDBs.Azure[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.AIMetricsClient

	// Parse Relative Time to absolute time
	timespan, err := timeSpan(e, sdur, edur)
	if err != nil {
		return nil, err
	}

	// Handle the timegrain (downsampling)
	var tg string
	if interval != "" {
		tg = *intervalToTimegrain(interval)
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
	seriesMap := make(map[string]expr.Series)

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
		expr.CollectCacheHit(e.Cache, "azureai_ts", hit)
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
					seriesMap[tags.Tags()] = make(expr.Series)
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
		r.Append(&expr.Element{
			Value: series,
			Group: tags,
		})
	}
	return r, nil
}

// AIListApps get a list of all applications on the subscription and returns those apps in a AzureApplicationInsightsApps within the result
func AIListApps(prefix string, e *expr.State) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
	// Verify prefix is a defined resource and fetch the collection of clients
	key := fmt.Sprintf("AzureAIAppCache:%s:%s", prefix, time.Now().Truncate(time.Minute*1)) // https://github.com/golang/groupcache/issues/92

	getFn := func() (interface{}, error) {
		cc, clientFound := e.TSDBs.Azure[prefix]
		if !clientFound {
			return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
		}
		c := cc.AIComponentsClient
		applist := tsdbs.AzureApplicationInsightsApps{Prefix: prefix}
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
				applist.Applications = append(applist.Applications, tsdbs.AzureApplicationInsightsApp{
					ApplicationName: *comp.Name,
					AppId:           *comp.ApplicationInsightsComponentProperties.AppID,
					Tags:            azTags,
				})
			}
		}
		r.Append(&expr.Element{Value: applist})
		return r, nil
	}
	val, err, hit := e.Cache.Get(key, getFn)
	expr.CollectCacheHit(e.Cache, "azure_aiapplist", hit)
	if err != nil {
		return r, err
	}
	return val.(*expr.ValueSet), nil
}

// AIMetricMD returns metric metadata for the listed AzureApplicationInsightsApps. This is not meant
// as core expression function, but rather one for interactive inspection through the expression UI.
func AIMetricMD(prefix string, e *expr.State, apps tsdbs.AzureApplicationInsightsApps) (r *expr.ValueSet, err error) {
	r = new(expr.ValueSet)
	if apps.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use apps from client "%v" on a query with client "%v"`, apps.Prefix, prefix)
	}
	cc, clientFound := e.TSDBs.Azure[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.AIMetricsClient
	for _, app := range apps.Applications {
		md, err := c.GetMetadata(context.Background(), app.AppId)
		if err != nil {
			return r, err
		}
		r.Append(&expr.Element{
			Value: expr.Info{md.Value},
			Group: opentsdb.TagSet{"app": app.ApplicationName},
		})
	}
	return
}

// aiTags is the tag function for the "az" expression function
func aiTags(args []parse.Node) (parse.TagKeys, error) {
	tags := parse.TagKeys{"app": struct{}{}}
	csvTags := strings.Split(args[1].(*parse.StringNode).Text, ",")
	if len(csvTags) == 1 && csvTags[0] == "" {
		return tags, nil
	}
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}
