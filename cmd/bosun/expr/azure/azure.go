package expr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"bosun.org/metadata"

	"bosun.org/collect"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/kylebrandt/boolq"
)

// ExprFuncs is the collection of functions for the Azure monitor datasource.
var ExprFuncs = map[string]parse.Func{
	"az": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		TagKeys:       azTags,
		F:             AzureQuery,
		PrefixEnabled: true,
	},
	"azmulti": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeAzureResourceList, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		TagKeys:       azMultiTags,
		F:             AzureMultiQuery,
		PrefixEnabled: true,
	},
	"azmd": { // TODO Finish and document this func
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet, // TODO return type
		TagKeys:       expr.TagFirst,        //TODO: Appropriate tags func
		F:             AzureMetricDefinitions,
		PrefixEnabled: true,
	},
	"azrt": {
		Args:          []models.FuncType{models.TypeString},
		Return:        models.TypeAzureResourceList,
		F:             AzureResourcesByType,
		PrefixEnabled: true,
	},
	"azrf": {
		Args:   []models.FuncType{models.TypeAzureResourceList, models.TypeString},
		Return: models.TypeAzureResourceList,
		F:      AzureFilterResources,
	},
	// Azure function for application insights, See azureai.go
	"aiapp": {
		Args:          []models.FuncType{},
		Return:        models.TypeAzureAIApps,
		F:             AzureAIListApps,
		PrefixEnabled: true,
	},
	"aiappf": {
		Args:          []models.FuncType{models.TypeAzureAIApps, models.TypeString},
		Return:        models.TypeAzureAIApps,
		F:             AzureAIFilterApps,
		PrefixEnabled: true,
	},
	"aimd": {
		Args:          []models.FuncType{models.TypeAzureAIApps},
		Return:        models.TypeInfo,
		F:             AzureAIMetricMD,
		PrefixEnabled: true,
	},
	"ai": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeAzureAIApps, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		TagKeys:       azAITags,
		F:             AzureAIQuery,
		PrefixEnabled: true,
	},
}

// azTags is the tag function for the "az" expression function
func azTags(args []parse.Node) (parse.TagKeys, error) {
	return azureTags(args[2])
}

// azMultiTag function for the "azmulti" expression function
func azMultiTags(args []parse.Node) (parse.TagKeys, error) {
	return azureTags(args[1])
}

// azureTags adds tags for the csv argument along with the "name" and "rsg" tags
func azureTags(arg parse.Node) (parse.TagKeys, error) {
	tags := parse.TagKeys{expr.AzureTagName: struct{}{}, expr.AzureTagRSG: struct{}{}}
	csvTags := strings.Split(arg.(*parse.StringNode).Text, ",")
	for _, k := range csvTags {
		tags[k] = struct{}{}
	}
	return tags, nil
}

// Azure API References
// - https://docs.microsoft.com/en-us/azure/monitoring-and-diagnostics/monitoring-supported-metrics
// - https://docs.microsoft.com/en-us/azure/monitoring-and-diagnostics/monitoring-data-sources

// TODO
// - Finish up azmd info function

const azTimeFmt = "2006-01-02T15:04:05"

// azResourceURI builds a resource uri appropriate for an Azure API request based on the arguments
func azResourceURI(subscription, resourceGrp, Namespace, Resource string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s", subscription, resourceGrp, Namespace, Resource)
}

// AzureMetricDefinitions fetches metric information for a specific resource and metric tuple
// TODO make this return and not fmt.Printf
func AzureMetricDefinitions(prefix string, e *expr.State, namespace, metric, rsg, resource string) (r *expr.Results, err error) {
	r = new(expr.Results)
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf("azure client with name %v not defined", prefix)
	}
	c := cc.MetricDefinitionsClient
	defs, err := c.List(context.Background(), azResourceURI(c.SubscriptionID, rsg, namespace, resource), namespace)
	if err != nil {
		return
	}
	if defs.Value == nil {
		return r, fmt.Errorf("No metric definitions in response")
	}
	for _, def := range *defs.Value {
		agtypes := []string{}
		for _, x := range *def.SupportedAggregationTypes {
			agtypes = append(agtypes, fmt.Sprintf("%s", x))
		}
		dims := []string{}
		if def.Dimensions != nil {
			for _, x := range *def.Dimensions {
				dims = append(dims, fmt.Sprintf("%s", *x.Value))
			}
		}
		fmt.Println(*def.Name.LocalizedValue, strings.Join(dims, ", "), strings.Join(agtypes, ", "))
	}
	return
}

func azureTimeSpan(e *expr.State, sdur, edur string) (span string, err error) {
	sd, err := opentsdb.ParseDuration(sdur)
	if err != nil {
		return
	}
	var ed opentsdb.Duration
	if edur != "" {
		ed, err = opentsdb.ParseDuration(edur)
		if err != nil {
			return
		}
	}
	st := e.Now().Add(time.Duration(-sd)).Format(azTimeFmt)
	en := e.Now().Add(time.Duration(-ed)).Format(azTimeFmt)
	return fmt.Sprintf("%s/%s", st, en), nil
}

// azureQuery queries Azure metrics for time series data based on the resourceUri
func azureQuery(prefix string, e *expr.State, metric, tagKeysCSV, rsg, resName, resourceUri, agtype, interval, sdur, edur string) (r *expr.Results, err error) {
	r = new(expr.Results)
	// Verify prefix is a defined resource and fetch the collection of clients
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.MetricsClient
	r = new(expr.Results)
	// Parse Relative Time to absolute time
	timespan, err := azureTimeSpan(e, sdur, edur)
	if err != nil {
		return nil, err
	}

	// Set Dimensions (tag) keys for metrics that support them by building an Azure filter
	// expression in form of "tagKey eq '*' and tagKey eq ..."
	// reference: https://docs.microsoft.com/en-us/rest/api/monitor/filter-syntax
	filter := ""
	if tagKeysCSV != "" {
		filters := []string{}
		tagKeys := strings.Split(tagKeysCSV, ",")
		for _, k := range tagKeys {
			filters = append(filters, fmt.Sprintf("%s eq '*'", k))
		}
		filter = strings.Join(filters, " and ")
	}

	// Set the Interval/Timegrain (Azure metric downsampling)
	var tg *string
	if interval != "" {
		tg = azureIntervalToTimegrain(interval)
	}

	// Set Azure aggregation method
	aggLong, err := azureShortAggToLong(agtype)
	if err != nil {
		return
	}
	cacheKey := strings.Join([]string{metric, filter, resourceUri, aggLong, interval, timespan}, ":")
	getFn := func() (interface{}, error) {
		req, err := c.ListPreparer(context.Background(), resourceUri,
			timespan,
			tg,
			metric,
			aggLong,
			nil,
			"",
			filter,
			insights.Data,
			"")
		if err != nil {
			return nil, err
		}
		var resp insights.Response
		e.Timer.StepCustomTiming("azure", "query", req.URL.String(), func() {
			hr, sendErr := c.ListSender(req)
			if sendErr == nil {
				resp, err = c.ListResponder(hr)
			} else {
				err = sendErr
			}
		})
		return resp, err
	}
	// Get Azure metric values by calling the Azure API or via cache if available
	val, err, hit := e.Cache.Get(cacheKey, getFn)
	if err != nil {
		return r, err
	}
	expr.CollectCacheHit(e.Cache, "azure_ts", hit)
	resp := val.(insights.Response)
	rawReadsRemaining := resp.Header.Get("X-Ms-Ratelimit-Remaining-Subscription-Reads")
	readsRemaining, err := strconv.ParseInt(rawReadsRemaining, 10, 64)
	if err != nil {
		slog.Errorf("failure to parse remaning reads from Azure response")
	} else {
		// Since we may be hitting different Azure Resource Manager servers on Azure's side the rate limit
		// may have a high variance therefore we sample
		// see https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-manager-request-limits
		collect.Sample("azure.remaining_reads", opentsdb.TagSet{"prefix": prefix}, float64(readsRemaining))
		if readsRemaining < 100 {
			slog.Warningf("less than 100 reads detected for the Azure api on client %v", prefix)
		}
	}
	if resp.Value != nil {
		for _, tsContainer := range *resp.Value {
			if tsContainer.Timeseries == nil {
				continue // If the container doesn't have a time series object then skip
			}
			for _, dataContainer := range *tsContainer.Timeseries {
				if dataContainer.Data == nil {
					continue // The timeseries has no data in it - then skip
				}
				series := make(expr.Series)
				tags := make(opentsdb.TagSet)
				tags[expr.AzureTagRSG] = rsg
				tags[expr.AzureTagName] = resName
				// Get the Key/Values that make up the Azure dimension and turn them into tags
				if dataContainer.Metadatavalues != nil {
					for _, md := range *dataContainer.Metadatavalues {
						if md.Name != nil && md.Name.Value != nil && md.Value != nil {
							tags[*md.Name.Value] = *md.Value
						}
					}
				}
				for _, mValue := range *dataContainer.Data {
					// extract the value that corresponds the the request aggregation
					exValue := azureExtractMetricValue(&mValue, aggLong)
					if exValue != nil && mValue.TimeStamp != nil {
						series[mValue.TimeStamp.ToTime()] = *exValue
					}
				}
				if len(series) == 0 {
					continue // If we end up with an empty series then skip
				}
				r.Results = append(r.Results, &expr.Result{
					Value: series,
					Group: tags,
				})
			}
		}
	}
	return r, nil
}

// AzureQuery queries an Azure monitor metric for the given resource and returns a series set tagged by
// rsg (resource group), name (resource name), and any tag keys parsed from the tagKeysCSV argument
func AzureQuery(prefix string, e *expr.State, namespace, metric, tagKeysCSV, rsg, resName, agtype, interval, sdur, edur string) (r *expr.Results, err error) {
	r = new(expr.Results)
	// Verify prefix is a defined resource and fetch the collection of clients
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.MetricsClient
	resourceURI := azResourceURI(c.SubscriptionID, rsg, namespace, resName)
	return azureQuery(prefix, e, metric, tagKeysCSV, rsg, resName, resourceURI, agtype, interval, sdur, edur)
}

// AzureMultiQuery queries multiple Azure resources and returns them as a single result set
// It makes one HTTP request per resource and parallelizes the requests
func AzureMultiQuery(prefix string, e *expr.State, metric, tagKeysCSV string, resources expr.AzureResources, agtype string, interval, sdur, edur string) (r *expr.Results, err error) {
	r = new(expr.Results)
	if resources.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use resources from client "%v" on a query with client "%v"`, resources.Prefix, prefix)
	}
	nResources := len(resources.Resources)
	if nResources == 0 {
		return r, nil
	}
	queryResults := []*expr.Results{}
	var wg sync.WaitGroup
	// reqCh (Request Channel) is populated with Azure resources, and resources are pulled from channel to make a time series request per resource
	reqCh := make(chan expr.AzureResource, nResources)
	// resCh (Result Channel) contains the timeseries responses for requests for resource
	resCh := make(chan *expr.Results, nResources)
	// errCh (Error Channel) contains any request errors
	errCh := make(chan error, nResources)
	// a worker makes a time series request for a resource
	worker := func() {
		for resource := range reqCh {
			res, err := azureQuery(prefix, e, metric, tagKeysCSV, resource.ResourceGroup, resource.Name, resource.ID, agtype, interval, sdur, edur)
			resCh <- res
			errCh <- err
		}
		defer wg.Done()
	}
	// Create N workers to parallelize multiple requests at once since he resource requires an HTTP request
	for i := 0; i < e.AzureMonitor[prefix].Concurrency; i++ {
		wg.Add(1)
		go worker()
	}
	timingString := fmt.Sprintf(`%v queries for metric:"%v" using client "%v"`, nResources, metric, prefix)
	e.Timer.StepCustomTiming("azure", "query-multi", timingString, func() {
		// Feed resources into the request channel which the workers will consume
		for _, resource := range resources.Resources {
			reqCh <- resource
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
	r, err = expr.Merge(e, queryResults...)
	return
}

// azureListResources fetches all resources for the tenant/subscription and caches them for
// up to one minute.
func azureListResources(prefix string, e *expr.State) (expr.AzureResources, error) {
	// Cache will only last for one minute. In practice this will only apply for web sessions since a
	// new cache is created for each check cycle in the cache
	key := fmt.Sprintf("AzureResourceCache:%s:%s", prefix, time.Now().Truncate(time.Minute*1)) // https://github.com/golang/groupcache/issues/92
	// getFn is a cacheable function for listing Azure resources
	getFn := func() (interface{}, error) {
		r := expr.AzureResources{Prefix: prefix}
		cc, clientFound := e.Backends.AzureMonitor[prefix]
		if !clientFound {
			return r, fmt.Errorf("Azure client with name %v not defined", prefix)
		}
		c := cc.ResourcesClient
		// Page through all resources
		for rList, err := c.ListComplete(context.Background(), "", "", nil); rList.NotDone(); err = rList.Next() {
			// TODO not catching auth error here for some reason, err is nil when error!!
			if err != nil {
				return r, err
			}
			val := rList.Value()
			if val.Name != nil && val.Type != nil && val.ID != nil {
				// Extract out the resource group name from the Id
				splitID := strings.Split(*val.ID, "/")
				if len(splitID) < 5 {
					return r, fmt.Errorf("unexpected ID for resource: %s", *val.ID)
				}
				// Add Azure tags to the resource
				azTags := make(map[string]string)
				for k, v := range val.Tags {
					if v != nil {
						azTags[k] = *v
					}
				}
				r.Resources = append(r.Resources, expr.AzureResource{
					Name:          *val.Name,
					Type:          *val.Type,
					ResourceGroup: splitID[4],
					Tags:          azTags,
					ID:            *val.ID,
				})
			}
		}
		return r, nil
	}
	val, err, hit := e.Cache.Get(key, getFn)
	expr.CollectCacheHit(e.Cache, "azure_resource", hit)
	if err != nil {
		return expr.AzureResources{}, err
	}
	return val.(expr.AzureResources), nil
}

// AzureResourcesByType returns all resources of the specified type
// It fetches the complete list resources and then filters them relying on a Cache of that resource list
func AzureResourcesByType(prefix string, e *expr.State, tp string) (r *expr.Results, err error) {
	resources := expr.AzureResources{Prefix: prefix}
	r = new(expr.Results)
	allResources, err := azureListResources(prefix, e)
	if err != nil {
		return
	}
	for _, res := range allResources.Resources {
		if res.Type == tp {
			resources.Resources = append(resources.Resources, res)
		}
	}
	r.Results = append(r.Results, &expr.Result{Value: resources})
	return
}

// AzureFilterResources filters a list of resources based on the value of the name, resource group
// or tags associated with that resource
func AzureFilterResources(e *expr.State, resources expr.AzureResources, filter string) (r *expr.Results, err error) {
	r = new(expr.Results)
	// Parse the filter once and then apply it to each item in the loop
	bqf, err := boolq.Parse(filter)
	if err != nil {
		return r, err
	}
	filteredResources := expr.AzureResources{Prefix: resources.Prefix}
	for _, res := range resources.Resources {
		match, err := boolq.AskParsedExpr(bqf, res)
		if err != nil {
			return r, err
		}
		if match {
			filteredResources.Resources = append(filteredResources.Resources, res)
		}
	}
	r.Results = append(r.Results, &expr.Result{Value: filteredResources})
	return
}

// AzureAIFilterApps filters a list of applications based on the name of the app, or the Azure tags associated with the application resource
func AzureAIFilterApps(prefix string, e *expr.State, apps expr.AzureApplicationInsightsApps, filter string) (r *expr.Results, err error) {
	r = new(expr.Results)
	// Parse the filter once and then apply it to each item in the loop
	bqf, err := boolq.Parse(filter)
	if err != nil {
		return r, err
	}
	filteredApps := expr.AzureApplicationInsightsApps{Prefix: apps.Prefix}
	for _, app := range apps.Applications {
		match, err := boolq.AskParsedExpr(bqf, app)
		if err != nil {
			return r, err
		}
		if match {
			filteredApps.Applications = append(filteredApps.Applications, app)
		}
	}
	r.Results = append(r.Results, &expr.Result{Value: filteredApps})
	return
}

// AzureExtractMetricValue is a helper for fetching the value of the requested
// aggregation for the metric
func azureExtractMetricValue(mv *insights.MetricValue, field string) (v *float64) {
	switch field {
	case string(insights.Average), "":
		v = mv.Average
	case string(insights.Minimum):
		v = mv.Minimum
	case string(insights.Maximum):
		v = mv.Maximum
	case string(insights.Total):
		v = mv.Total
	}
	return
}

// azureShortAggToLong coverts bosun style names for aggregations (like the reduction functions)
// to the string that is expected for Azure queries
func azureShortAggToLong(agtype string) (string, error) {
	switch agtype {
	case "avg", "":
		return string(insights.Average), nil
	case "min":
		return string(insights.Minimum), nil
	case "max":
		return string(insights.Maximum), nil
	case "total":
		return string(insights.Total), nil
	case "count":
		return string(insights.Count), nil
	}
	return "", fmt.Errorf("unrecognized aggregation type %s, must be avg, min, max, or total", agtype)
}

// azureIntervalToTimegrain adds a PT prefix and upper cases the argument to
// make the string in the format of Azure Timegrain
func azureIntervalToTimegrain(s string) *string {
	tg := fmt.Sprintf("PT%v", strings.ToUpper(s))
	return &tg
}

func init() {
	metadata.AddMetricMeta("bosun.azure.remaining_reads", metadata.Gauge, metadata.Operation,
		"A sampling of the number of remaining reads to the Azure API before being ratelimited.")
}
