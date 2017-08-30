package collectors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/bosun-monitor/statusio"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, f := range c.Fastly {
			client := newFastlyClient(f.Key)
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_fastly(client)
				},
				name:     "c_fastly",
				Interval: time.Minute * 1,
			})
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_fastly_billing(client)
				},
				name:     "c_fastly_billing",
				Interval: time.Minute * 5,
			})
			if f.StatusBaseAddr != "" {
				collectors = append(collectors, &IntervalCollector{
					F: func() (opentsdb.MultiDataPoint, error) {
						return c_fastly_status(f.StatusBaseAddr)
					},
					name:     "c_fastly_status",
					Interval: time.Minute * 1,
				})
			}
		}
	})
}

const (
	fastlyBillingPrefix             = "fastly.billing."
	fastlyBillingBandwidthDesc      = "The total amount of bandwidth used this month."
	fastlyBillingBandwidthCostDesc  = "The cost of the bandwidth used this month in USD."
	fastlyBillingRequestsDesc       = "The total number of requests used this month."
	fastlyBillingRequestsCostDesc   = "The cost of the requests used this month."
	fastlyBillingIncurredCostDesc   = "The total cost of bandwidth and requests used this month."
	fastlyBillingOverageDesc        = "How much over the plan minimum has been incurred this month."
	fastlyBillingExtrasCostDesc     = "The total cost of all extras this month."
	fastlyBillingBeforeDiscountDesc = "The total incurred cost plus extras cost this month."
	fastlyBillingDiscountDesc       = "The calculated discount rate this month."
	fastlyBillingCostDesc           = "The final amount to be paid this month."

	fastlyStatusPrefix             = "fastly.status."
	fastlyComponentStatusDesc      = "The current status of the %v. 0: Operational, 1: Degraded Performance, 2: Partial Outage, 3: Major Outage." // see iota for statusio.ComponentStatus
	fastlyScheduledMaintDesc       = "The number of currently scheduled maintenances. Does not include maintenance that is current active"
	fastlyActiveScheduledMaintDesc = "The number of currently scheduled maintenances currently in progress. Includes the 'in_progress' and 'verifying'"
	fastlyActiveIncidentDesc       = "The number of currently active incidents. Includes the 'investingating', 'identified', and 'monitoring' states."
)

var (
	fastlyStatusPopRegex = regexp.MustCompile(`(.*)\(([A-Z]{3})\)`) // i.e. Miami (MIA)
)

func c_fastly_status(baseAddr string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	c := statusio.NewClient(baseAddr)
	summary, err := c.GetSummary()
	if err != nil {
		return md, err
	}

	// Process Components (Pops, Support Systems)
	for _, comp := range summary.Components {
		match := fastlyStatusPopRegex.FindAllStringSubmatch(comp.Name, 1)
		if len(match) != 0 && len(match[0]) == 3 { // We have a pop
			//name := match[0][1]
			code := match[0][2]
			tagSet := opentsdb.TagSet{"code": code}
			Add(&md, fastlyStatusPrefix+"pop", int(comp.Status), tagSet, metadata.Gauge, metadata.StatusCode, fmt.Sprintf(fastlyComponentStatusDesc, "pop"))
			continue
		}
		// Must be service component
		tagSet := opentsdb.TagSet{"service": comp.Name}
		Add(&md, fastlyStatusPrefix+"service", int(comp.Status), tagSet, metadata.Gauge, metadata.StatusCode, fmt.Sprintf(fastlyComponentStatusDesc, "service"))
	}

	// Scheduled Maintenance
	scheduledMaintByImpact := make(map[statusio.StatusIndicator]int)
	activeScheduledMaintByImpact := make(map[statusio.StatusIndicator]int)
	// Make Maps
	for _, si := range statusio.StatusIndicatorValues {
		scheduledMaintByImpact[si] = 0
		activeScheduledMaintByImpact[si] = 0
	}
	// Group by scheduled vs inprogress/verifying
	for _, maint := range summary.ScheduledMaintenances {
		switch maint.Status {
		case statusio.Scheduled:
			scheduledMaintByImpact[maint.Impact]++
		case statusio.InProgress, statusio.Verifying:
			activeScheduledMaintByImpact[maint.Impact]++
		}
	}
	for impact, count := range scheduledMaintByImpact {
		tagSet := opentsdb.TagSet{"impact": fmt.Sprint(impact)}
		Add(&md, fastlyStatusPrefix+"scheduled_maint_count", count, tagSet, metadata.Gauge, metadata.Count, fastlyScheduledMaintDesc)
	}
	for impact, count := range activeScheduledMaintByImpact {
		tagSet := opentsdb.TagSet{"impact": fmt.Sprint(impact)}
		Add(&md, fastlyStatusPrefix+"in_progress_maint_count", count, tagSet, metadata.Gauge, metadata.Count, fastlyActiveScheduledMaintDesc)
	}

	// Incidents
	// Make Map
	incidentsByImpact := make(map[statusio.StatusIndicator]int)
	for _, si := range statusio.StatusIndicatorValues {
		incidentsByImpact[si] = 0
	}
	for _, incident := range summary.Incidents {
		switch incident.Status {
		case statusio.Investigating, statusio.Identified, statusio.Monitoring:
			incidentsByImpact[incident.Impact]++
		default:
			continue
		}
	}
	for impact, count := range incidentsByImpact {
		tagSet := opentsdb.TagSet{"impact": fmt.Sprint(impact)}
		Add(&md, fastlyStatusPrefix+"active_incident_count", count, tagSet, metadata.Gauge, metadata.Incident, fastlyActiveIncidentDesc)
	}

	return md, nil
}

func c_fastly_billing(c fastlyClient) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	now := time.Now().UTC()
	year := now.Format("2006")
	month := now.Format("01")
	b, err := c.GetBilling(year, month)
	if err != nil {
		return md, err
	}
	Add(&md, fastlyBillingPrefix+"bandwidth", b.Total.Bandwidth, nil, metadata.Gauge, metadata.Unit(b.Total.BandwidthUnits), fastlyBillingBandwidthDesc)
	Add(&md, fastlyBillingPrefix+"bandwidth_cost", b.Total.BandwidthCost, nil, metadata.Gauge, metadata.USD, fastlyBillingBandwidthCostDesc)
	Add(&md, fastlyBillingPrefix+"requests", b.Total.Requests, nil, metadata.Gauge, metadata.Request, fastlyBillingRequestsDesc)
	Add(&md, fastlyBillingPrefix+"requests_cost", b.Total.RequestsCost, nil, metadata.Gauge, metadata.USD, fastlyBillingRequestsCostDesc)
	Add(&md, fastlyBillingPrefix+"incurred_cost", b.Total.IncurredCost, nil, metadata.Gauge, metadata.USD, fastlyBillingIncurredCostDesc)
	Add(&md, fastlyBillingPrefix+"overage", b.Total.Overage, nil, metadata.Gauge, metadata.Unit("unknown"), fastlyBillingOverageDesc)
	Add(&md, fastlyBillingPrefix+"extras_cost", b.Total.ExtrasCost, nil, metadata.Gauge, metadata.USD, fastlyBillingExtrasCostDesc)
	Add(&md, fastlyBillingPrefix+"cost_before_discount", b.Total.CostBeforeDiscount, nil, metadata.Gauge, metadata.USD, fastlyBillingBeforeDiscountDesc)
	Add(&md, fastlyBillingPrefix+"discount", b.Total.Discount, nil, metadata.Gauge, metadata.Pct, fastlyBillingDiscountDesc)
	Add(&md, fastlyBillingPrefix+"cost", b.Total.Cost, nil, metadata.Gauge, metadata.USD, fastlyBillingCostDesc)

	return md, nil
}

func c_fastly(c fastlyClient) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	to := time.Now().UTC().Truncate(time.Minute)
	from := to.Add(-15 * time.Minute) // "Minutely data will be delayed by roughly 10 to 15 minutes from the current time -- Fastly Docs"

	// Aggregate
	statsCollection, err := c.GetAggregateStats(from, to)
	if err != nil {
		return md, err
	}
	for _, stats := range statsCollection {
		fastlyReflectAdd(&md, "fastly", "", stats, stats.StartTime, nil)
	}

	// By Service
	services, err := c.GetServices()
	if err != nil {
		return md, err
	}
	for _, service := range services {
		statsCollection, err := c.GetServiceStats(from, to, service.Id)
		if err != nil {
			slog.Errorf("couldn't get stats for service %v with id %v: %v", service.Name, service.Id, err)
			continue
		}
		for _, stats := range statsCollection {
			fastlyReflectAdd(&md, "fastly", "_by_service", stats, stats.StartTime, service.TagSet())
		}
	}

	// By Region
	regions, err := c.GetRegions()
	if err != nil {
		return md, err
	}
	for _, region := range regions {
		statsCollection, err := c.GetRegionStats(from, to, region)
		if err != nil {
			slog.Errorf("couldn't get stats for region %v: %v", region, err)
			continue
		}
		for _, stats := range statsCollection {
			fastlyReflectAdd(&md, "fastly", "_by_region", stats, stats.StartTime, region.TagSet())
		}
	}
	return md, nil
}

type fastlyClient struct {
	key    string
	client *http.Client
}

func newFastlyClient(key string) fastlyClient {
	return fastlyClient{key, &http.Client{}}
}

func (f *fastlyClient) request(path string, values url.Values, s interface{}) error {
	u := &url.URL{
		Scheme:   "https",
		Host:     "api.fastly.com",
		Path:     path,
		RawQuery: values.Encode(),
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		slog.Error(err)
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Fastly-Key", f.key)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%v: %v: %v", req.URL, resp.Status, string(b))
	}
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(&s); err != nil {
		return err
	}
	return nil
}

func (f *fastlyClient) GetServices() ([]fastlyService, error) {
	var services []fastlyService
	err := f.request("service", url.Values{}, &services)
	return services, err
}

type fastlyService struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

func (f *fastlyService) TagSet() opentsdb.TagSet {
	return opentsdb.TagSet{"service": f.Name}
}

type fastlyRegion string

func (fr fastlyRegion) TagSet() opentsdb.TagSet {
	return opentsdb.TagSet{"region": string(fr)}
}

func (f *fastlyClient) GetRegions() ([]fastlyRegion, error) {
	r := struct {
		Data []fastlyRegion `json:"data"`
	}{
		[]fastlyRegion{},
	}
	err := f.request("stats/regions", url.Values{}, &r)
	return r.Data, err
}

type fastlyBilling struct {
	// There are other breakdowns in the API response for by service and by regions. So this
	// can be expanded in the future if we wish
	Total struct {
		Bandwidth          float64       `json:"bandwidth"`
		BandwidthCost      float64       `json:"bandwidth_cost"`
		BandwidthUnits     string        `json:"bandwidth_units"`
		Cost               float64       `json:"cost"`
		CostBeforeDiscount float64       `json:"cost_before_discount"`
		Discount           float64       `json:"discount"`
		Extras             []interface{} `json:"extras"`
		ExtrasCost         float64       `json:"extras_cost"`
		Overage            float64       `json:"overage"`
		IncurredCost       float64       `json:"incurred_cost"`
		PlanCode           string        `json:"plan_code"`
		PlanMinimum        float64       `json:"plan_minimum"`
		PlanName           string        `json:"plan_name"`
		Requests           float64       `json:"requests"`
		RequestsCost       float64       `json:"requests_cost"`
		Terms              string        `json:"terms"`
	} `json:"total"`
}

func (f *fastlyClient) GetBilling(year, month string) (fastlyBilling, error) {
	var b fastlyBilling
	err := f.request(fmt.Sprintf("billing/year/%v/month/%v", year, month), nil, &b)
	return b, err
}

func (f *fastlyClient) GetAggregateStats(from, to time.Time) ([]fastlyStats, error) {
	v := url.Values{}
	v.Add("from", fmt.Sprintf("%v", from.Unix()))
	v.Add("to", fmt.Sprintf("%v", to.Unix()))
	v.Add("by", "minute")
	r := struct {
		Data []fastlyStats `json:"data"`
	}{
		[]fastlyStats{},
	}
	err := f.request("stats/aggregate", v, &r)
	return r.Data, err
}

func (f *fastlyClient) GetServiceStats(from, to time.Time, serviceId string) ([]fastlyStats, error) {
	v := url.Values{}
	v.Add("from", fmt.Sprintf("%v", from.Unix()))
	v.Add("to", fmt.Sprintf("%v", to.Unix()))
	v.Add("by", "minute")
	r := struct {
		Data []fastlyStats `json:"data"`
	}{
		[]fastlyStats{},
	}
	err := f.request(fmt.Sprintf("stats/service/%v", serviceId), v, &r)
	return r.Data, err
}

func (f *fastlyClient) GetRegionStats(from, to time.Time, region fastlyRegion) ([]fastlyStats, error) {
	v := url.Values{}
	v.Add("from", fmt.Sprintf("%v", from.Unix()))
	v.Add("to", fmt.Sprintf("%v", to.Unix()))
	v.Add("by", "minute")
	v.Add("region", string(region))
	r := struct {
		Data []fastlyStats `json:"data"`
	}{
		[]fastlyStats{},
	}
	err := f.request("stats/aggregate", v, &r)
	return r.Data, err
}

// Stats without a description are not sent as it means I wasn't able to find documentation on them
type fastlyStats struct {
	AttackBlock        int64   `json:"attack_block"`
	AttackBodySize     int64   `json:"attack_body_size"`
	AttackHeaderSize   int64   `json:"attack_header_size"`
	AttackSynth        int64   `json:"attack_synth"`
	Bandwidth          int64   `json:"bandwidth" div:"true" rate:"gauge" unit:"bytes per second" desc:"The total bytes delivered per second (body_size + header_size)."`
	Blacklist          int64   `json:"blacklist"`
	BodySize           int64   `json:"body_size" div:"true" rate:"gauge" unit:"bytes per second" desc:"The total bytes delivered per second for the bodies."`
	Errors             int64   `json:"errors" div:"true" rate:"gauge" unit:"errors per second" desc:"The number of cache errors per second."`
	HeaderSize         int64   `json:"header_size" div:"true" rate:"gauge" unit:"bytes per second" desc:"The total bytes delivered per second for headers."`
	HitRatio           float64 `json:"hit_ratio" rate:"gauge" unit:"ratio" desc:"The ratio of cache hits to cache misses (between 0-1)."`
	Hits               int64   `json:"hits" div:"true" rate:"gauge" unit:"hits per second" desc:"The number of cache hits per second."`
	HitsTime           float64 `json:"hits_time" div:"true" rate:"gauge" unit:"seconds per second" desc:"The amount of time spent processing cache hits."`
	HTTP2              int64   `json:"http2"`
	Imgopto            int64   `json:"imgopto"`
	Ipv6               int64   `json:"ipv6"`
	Log                int64   `json:"log"`
	MissTime           float64 `json:"miss_time" div:"true" rate:"gauge" unit:"seconds per second" desc:"The amount of time spent processing cache misses"`
	OrigReqBodySize    int64   `json:"orig_req_body_size"`
	OrigReqHeaderSize  int64   `json:"orig_req_header_size"`
	OrigRespBodySize   int64   `json:"orig_resp_body_size"`
	OrigRespHeaderSize int64   `json:"orig_resp_header_size"`
	Otfp               int64   `json:"otfp"`
	Pass               int64   `json:"pass" div:"true" rate:"gauge" unit:"hits per second" desc:"The number of requests that passed through the CDN without being cached"`
	Pci                int64   `json:"pci"`
	Requests           int64   `json:"requests" div:"true" rate:"gauge" unit:"requests per second" desc:"The number of requests processed"`
	Shield             int64   `json:"shield"`
	StartTime          int64   `json:"start_time" exclude:""`
	Status1xx          int64   `json:"status_1xx" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 1xx response code (Informational)."`
	Status200          int64   `json:"status_200" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 200 response code (Success)."`
	Status204          int64   `json:"status_204" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 204 response code (No Content)."`
	Status2xx          int64   `json:"status_2xx" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 2xx response code (Success)."`
	Status301          int64   `json:"status_301" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 301 response code (Moved Permanently)."`
	Status302          int64   `json:"status_302" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 302 response code (Found)."`
	Status304          int64   `json:"status_304" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 304 response code."`
	Status3xx          int64   `json:"status_3xx" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 3xx response code (Redirection)."`
	Status4xx          int64   `json:"status_4xx" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 4xx response code (Client Error)."`
	Status503          int64   `json:"status_503" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a 503 response code. (Service Unavailable)"`
	Status5xx          int64   `json:"status_5xx" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http responses delivered with a xxx response code."`
	Synth              int64   `json:"synth" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of synthetic responses sent from Varnish. This is typically used to send edge-generated error pages."`
	TLS                int64   `json:"tls"`
	Uncacheable        int64   `json:"uncacheable" div:"true" rate:"gauge" unit:"responses per second" desc:"The number of http requests that were uncacheable."`
	Video              int64   `json:"video"`
}

const fastlyDivDesc = "This metric is collected per minute, but we divide it by 60 seconds in order to normalize the rate to per second instead of per minute."

func fastlyReflectAdd(md *opentsdb.MultiDataPoint, prefix, suffix string, st interface{}, timeStamp int64, ts opentsdb.TagSet) {
	t := reflect.TypeOf(st)
	valueOf := reflect.ValueOf(st)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := valueOf.Field(i).Interface()
		var (
			jsonTag   = field.Tag.Get("json")
			metricTag = field.Tag.Get("metric")
			rateTag   = field.Tag.Get("rate")
			unitTag   = field.Tag.Get("unit")
			divTag    = field.Tag.Get("div")
			descTag   = field.Tag.Get("desc")
			exclude   = field.Tag.Get("exclude") != ""
		)
		if exclude || descTag == "" {
			continue
		}
		metricName := jsonTag
		if metricTag != "" {
			metricName = metricTag
		}
		if metricName == "" {
			slog.Errorf("Unable to determine metric name for field %s. Skipping.", field.Name)
			continue
		}
		shouldDiv := divTag != ""
		if shouldDiv {
			descTag = fmt.Sprintf("%v %v", descTag, fastlyDivDesc)
		}
		fullMetric := fmt.Sprintf("%v.%v%v", prefix, metricName, suffix)
		switch value := value.(type) {
		case int64, float64:
			var v float64
			if f, found := value.(float64); found {
				v = f
			} else {
				v = float64(value.(int64))
			}
			if shouldDiv {
				v /= 60.0
			}
			AddTS(md, fullMetric, timeStamp, v, ts, metadata.RateType(rateTag), metadata.Unit(unitTag), descTag)
		case string:
			// Floats in strings, I know not why, precision perhaps?
			// err ignored since we expect non number strings in the struct
			if f, err := strconv.ParseFloat(value, 64); err != nil {
				if shouldDiv {
					f /= 60.0
				}
				AddTS(md, fullMetric, timeStamp, f, ts, metadata.RateType(rateTag), metadata.Unit(unitTag), descTag)
			}
		default:
			// Pass since there is no need to recurse
		}
	}
}
