package expr

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"bosun.org/metadata"

	"bosun.org/collect"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/kylebrandt/boolq"
)

// AzureMonitor is the collection of functions for the Azure monitor datasource
var AzureMonitor = map[string]parse.Func{
	"az": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          azTags,
		F:             AzureQuery,
		PrefixEnabled: true,
	},
	"azmulti": {
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeAzureResourceList, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          azMultiTags,
		F:             AzureMultiQuery,
		PrefixEnabled: true,
	},
	"azmd": { // TODO Finish and document this func
		Args:          []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet, // TODO return type
		Tags:          tagFirst,             //TODO: Appropriate tags func
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
}

// azTags is the tag function for the "az" expression function
func azTags(args []parse.Node) (parse.Tags, error) {
	return azureTags(args[2])
}

// azMultiTag function for the "azmulti" expression function
func azMultiTags(args []parse.Node) (parse.Tags, error) {
	return azureTags(args[1])
}

// azureTags adds tags for the csv argument along with the "name" and "rsg" tags
func azureTags(arg parse.Node) (parse.Tags, error) {
	tags := parse.Tags{azureTagName: struct{}{}, azureTagRSG: struct{}{}}
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
func AzureMetricDefinitions(prefix string, e *State, namespace, metric, rsg, resource string) (r *Results, err error) {
	r = new(Results)
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

// azureQuery queries Azure metrics for time series data based on the resourceUri
func azureQuery(prefix string, e *State, metric, tagKeysCSV, rsg, resName, resourceUri, agtype, interval, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	// Verify prefix is a defined resource and fetch the collection of clients
	cc, clientFound := e.Backends.AzureMonitor[prefix]
	if !clientFound {
		return r, fmt.Errorf(`azure client with name "%v" not defined`, prefix)
	}
	c := cc.MetricsClient
	r = new(Results)
	// Parse Relative Time to absolute time
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
	st := e.now.Add(time.Duration(-sd)).Format(azTimeFmt)
	en := e.now.Add(time.Duration(-ed)).Format(azTimeFmt)

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
	cacheKey := strings.Join([]string{metric, filter, resourceUri, aggLong, interval, st, en}, ":")
	getFn := func() (interface{}, error) {
		req, err := c.ListPreparer(context.Background(), resourceUri,
			fmt.Sprintf("%s/%s", st, en),
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
	collectCacheHit(e.Cache, "azure_ts", hit)
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
				series := make(Series)
				tags := make(opentsdb.TagSet)
				tags[azureTagRSG] = rsg
				tags[azureTagName] = resName
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
				r.Results = append(r.Results, &Result{
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
func AzureQuery(prefix string, e *State, namespace, metric, tagKeysCSV, rsg, resName, agtype, interval, sdur, edur string) (r *Results, err error) {
	r = new(Results)
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
func AzureMultiQuery(prefix string, e *State, metric, tagKeysCSV string, resources AzureResources, agtype string, interval, sdur, edur string) (r *Results, err error) {
	r = new(Results)
	if resources.Prefix != prefix {
		return r, fmt.Errorf(`mismatched Azure clients: attempting to use resources from client "%v" on a query with client "%v"`, resources.Prefix, prefix)
	}
	nResources := len(resources.Resources)
	if nResources == 0 {
		return r, nil
	}
	queryResults := []*Results{}
	var wg sync.WaitGroup
	// reqCh (Request Channel) is populated with Azure resources, and resources are pulled from channel to make a time series request per resource
	reqCh := make(chan AzureResource, nResources)
	// resCh (Result Channel) contains the timeseries responses for requests for resource
	resCh := make(chan *Results, nResources)
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
	r, err = Merge(e, queryResults...)
	return
}

// azureListResources fetches all resources for the tenant/subscription and caches them for
// up to one minute.
func azureListResources(prefix string, e *State) (AzureResources, error) {
	// Cache will only last for one minute. In practice this will only apply for web sessions since a
	// new cache is created for each check cycle in the cache
	key := fmt.Sprintf("AzureResourceCache:%s:%s", prefix, time.Now().Truncate(time.Minute*1)) // https://github.com/golang/groupcache/issues/92
	// getFn is a cacheable function for listing Azure resources
	getFn := func() (interface{}, error) {
		r := AzureResources{Prefix: prefix}
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
				r.Resources = append(r.Resources, AzureResource{
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
	collectCacheHit(e.Cache, "azure_resource", hit)
	if err != nil {
		return AzureResources{}, err
	}
	return val.(AzureResources), nil
}

// AzureResourcesByType returns all resources of the specified type
// It fetches the complete list resources and then filters them relying on a Cache of that resource list
func AzureResourcesByType(prefix string, e *State, tp string) (r *Results, err error) {
	resources := AzureResources{Prefix: prefix}
	r = new(Results)
	allResources, err := azureListResources(prefix, e)
	if err != nil {
		return
	}
	for _, res := range allResources.Resources {
		if res.Type == tp {
			resources.Resources = append(resources.Resources, res)
		}
	}
	r.Results = append(r.Results, &Result{Value: resources})
	return
}

// AzureFilterResources filters a list of resources based on the value of the name, resource group
// or tags associated with that resource
func AzureFilterResources(e *State, resources AzureResources, filter string) (r *Results, err error) {
	r = new(Results)
	// Parse the filter once and then apply it to each item in the loop
	bqf, err := boolq.Parse(filter)
	if err != nil {
		return r, err
	}
	filteredResources := AzureResources{Prefix: resources.Prefix}
	for _, res := range resources.Resources {
		match, err := boolq.AskParsedExpr(bqf, res)
		if err != nil {
			return r, err
		}
		if match {
			filteredResources.Resources = append(filteredResources.Resources, res)
		}
	}
	r.Results = append(r.Results, &Result{Value: filteredResources})
	return
}

// AzureResource is a container for Azure resource information that Bosun can interact with
type AzureResource struct {
	Name          string
	Type          string
	ResourceGroup string
	Tags          map[string]string
	ID            string
}

// AzureResources is a slice of AzureResource
//type AzureResources []AzureResource
type AzureResources struct {
	Resources []AzureResource
	Prefix    string
}

// Ask makes an AzureResource a github.com/kylebrandt/boolq Asker, which allows it to
// to take boolean expressions to create conditions
func (ar AzureResource) Ask(filter string) (bool, error) {
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
		if re.MatchString(ar.Name) {
			return true, nil
		}
	case azureTagRSG:
		re, err := regexp.Compile(value)
		if err != nil {
			return false, err
		}
		if re.MatchString(ar.ResourceGroup) {
			return true, nil
		}
	default:
		// Does not support tags that have a tag key of rsg, resourcegroup, or name. If it is a problem at some point
		// we can do something like "\name" to mean the tag "name" if such thing is even allowed
		if tagV, ok := ar.Tags[key]; ok {
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

// AzureMonitorClientCollection is a collection of Azure SDK clients since
// the SDK provides different clients to access different sorts of resources
type AzureMonitorClientCollection struct {
	MetricsClient           insights.MetricsClient
	MetricDefinitionsClient insights.MetricDefinitionsClient
	ResourcesClient         resources.Client
	Concurrency             int
}

// AzureMonitorClients is map of all the AzureMonitorClientCollections that
// have been configured. This is so multiple subscription/tenant/clients
// can be queries from the same Bosun instance using the prefix syntax
type AzureMonitorClients map[string]AzureMonitorClientCollection

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

const (
	// constants for tag keys
	azureTagName = "name"
	azureTagRSG  = "rsg"
)
