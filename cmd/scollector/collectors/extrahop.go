package collectors

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/kylebrandt/gohop"
)

const extraHopIntervalSeconds int = 30

var extraHopFilterProtoBy string       //What to filter the traffic by. Valid values are "namedprotocols", "toppercent" or "none"
var extraHopTopProtoPerc int           //Only log the top % of protocols by volume
var extraHopOtherProtoName string      //What name to log the "other" data under.
var extraHopL7Description string       //What to append to the end of the L7 description metadata to explain what is and isn't filtered out
var extraHopAdditionalMetrics []string //Other metrics to fetch from Extrahop
var extraHopCertificateMatch *regexp.Regexp
var extraHopCertificateActivityGroup int

// ExtraHop collection registration
func ExtraHop(host, apikey, filterby string, filterpercent int, customMetrics []string, certMatch string, certActivityGroup int) error {
	if host == "" || apikey == "" {
		return fmt.Errorf("empty host or API key for ExtraHop")
	}

	extraHopAdditionalMetrics = customMetrics
	extraHopFilterProtoBy = filterby
	switch filterby { //Set up options
	case "toppercent":
		extraHopL7Description = fmt.Sprintf("Only the top %d percent of traffic has its protocols logged, the remainder is tagged as as proto=otherprotos", extraHopTopProtoPerc)
		extraHopOtherProtoName = "otherprotos"
		if filterpercent > 0 && filterpercent < 100 {
			extraHopTopProtoPerc = filterpercent
		} else {
			return fmt.Errorf("invalid ExtraHop FilterPercent value (%d). Number should be between 1 and 99", filterpercent)
		}
	case "namedprotocols":
		extraHopL7Description = "Only named protocols are logged. Any unnamed protocol (A protocol name starting with tcp, udp or ssl) is tagged as proto=unnamed"
		extraHopOtherProtoName = "unnamed"
	//There is also case "none", but in that case the options we need to keep as default, so there's actually nothing to do here.
	default:
		return fmt.Errorf("invalid ExtraHop FilterBy option (%s). Valid options are namedprotocols, toppercent or none", filterby)

	}
	//Add the metadata for the L7 types, as now we have enough information to know what they're going to be
	for l7type, l7s := range l7types {
		xhMetricName := fmt.Sprintf("extrahop.l7.%s", l7type)
		metadata.AddMeta(xhMetricName, nil, "rate", l7s.Rate, false)
		metadata.AddMeta(xhMetricName, nil, "unit", l7s.Unit, false)
		metadata.AddMeta(xhMetricName, nil, "desc", fmt.Sprintf("%s %s", l7s.Description, extraHopL7Description), false)
	}
	u, err := url.Parse(host)
	if err != nil {
		return err
	}

	if certMatch != "" {
		compiledRegexp, err := regexp.Compile(certMatch)
		if err != nil {
			return err
		}
		extraHopCertificateMatch = compiledRegexp
		extraHopCertificateActivityGroup = certActivityGroup
	}
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_extrahop(host, apikey)
		},
		name:     fmt.Sprintf("extrahop-%s", u.Host),
		Interval: time.Second * time.Duration(extraHopIntervalSeconds),
	})
	return nil

}

func c_extrahop(host, apikey string) (opentsdb.MultiDataPoint, error) {
	c := gohop.NewClient(host, apikey)
	var md opentsdb.MultiDataPoint
	if err := extraHopNetworks(c, &md); err != nil {
		return nil, err
	}
	if err := extraHopGetAdditionalMetrics(c, &md); err != nil {
		return nil, err
	}
	if err := extraHopGetCertificates(c, &md); err != nil {
		return nil, err
	}

	return md, nil
}

func extraHopGetAdditionalMetrics(c *gohop.Client, md *opentsdb.MultiDataPoint) error {
	for _, v := range extraHopAdditionalMetrics {
		metric, err := gohop.StoEHMetric(v)
		if err != nil {
			return err
		}
		ms := []gohop.MetricSpec{ //Build a metric spec to tell ExtraHop what we want to pull out.
			{Name: metric.MetricSpecName, CalcType: metric.MetricSpecCalcType, KeyPair: gohop.KeyPair{Key1Regex: "", Key2Regex: "", OpenTSDBKey1: "proto", Key2OpenTSDBKey2: ""}, OpenTSDBMetric: ehMetricNameEscape(v)},
		}
		mrk, err := c.KeyedMetricQuery(gohop.Cycle30Sec, metric.MetricCategory, metric.ObjectType, -60000, 0, ms, []int64{metric.ObjectId})
		if err != nil {
			return err
		}

		//This is our function that is going to be executed on each data point in the extrahop dataset
		appendMetricPoint := func(c *gohop.Client, md *opentsdb.MultiDataPoint, a *gohop.MetricStatKeyed, b *[]gohop.MetricStatKeyedValue, d *gohop.MetricStatKeyedValue) {
			switch d.Vtype {
			case "tset":
				for _, e := range d.Tset {
					*md = append(*md, &opentsdb.DataPoint{
						Metric:    ehMetricNameEscape(d.Key.Str),
						Timestamp: a.Time,
						Value:     e.Value,
						Tags:      ehItemNameToTagSet(c, e.Key.Str),
					})
				}
			}
		}

		processGohopStat(&mrk, c, md, appendMetricPoint) //This will loop through our datapoint structure and execute appendCountPoints on each final data piece
	}

	return nil
}

// extraHopNetworks grabs the complex metrics of the L7 traffic from ExtraHop. It is a complex type because the data is not just a simple time series,
// the data needs to be tagged with vlan, protocol, etc. We can do the network and vlan tagging ourselves, but the protocol tagging comes
// from ExtraHop itself.
func extraHopNetworks(c *gohop.Client, md *opentsdb.MultiDataPoint) error {
	nl, err := c.GetNetworkList(true) //Fetch the network list from ExtraHop, and include VLAN information
	if err != nil {
		return err
	}
	for _, net := range nl { //All found networks
		for _, vlan := range net.Vlans { //All vlans inside this network
			for l7type := range l7types { //All the types of data we want to retrieve for the vlan
				xhMetricName := fmt.Sprintf("extrahop.l7.%s", l7type)
				metricsDropped, metricsKept := 0, 0  //Counters for debugging purposes
				otherValues := make(map[int64]int64) //Container to put any extra time series data that we need to add, for consolidating unnamed or dropped protocols, etc.
				ms := []gohop.MetricSpec{            //Build a metric spec to tell ExtraHop what we want to grab from ExtraHop
					{Name: l7type, KeyPair: gohop.KeyPair{Key1Regex: "", Key2Regex: "", OpenTSDBKey1: "proto", Key2OpenTSDBKey2: ""}, OpenTSDBMetric: xhMetricName}, //ExtraHop breaks this by L7 protocol on its own, but we need to tell TSDB what tag to add, which is in this case "proto"
				}
				mrk, err := c.KeyedMetricQuery(gohop.Cycle30Sec, "app", "vlan", int64(extraHopIntervalSeconds)*-1000, 0, ms, []int64{vlan.VlanId}) //Get the data from ExtraHop
				if err != nil {
					return err
				}
				md2, err := mrk.OpenTSDBDataPoints(ms, "vlan", map[int64]string{vlan.VlanId: fmt.Sprintf("%d", vlan.VlanId)}) //Get the OpenTSDBDataPoints from the ExtraHop data
				if err != nil {
					return err
				}
				valueCutoff := calculateDataCutoff(mrk) //Calculate what the cutoff value will be (used later on when we decide whether or not to consolidate the data)
				for _, dp := range md2 {                //We need to manually process the TSDB datapoints that we've got
					dp.Tags["host"] = c.APIHost
					dp.Tags["network"] = net.Name
					switch extraHopFilterProtoBy { //These are our filter options from the the configuration file. Filter by %, named, or none
					case "toppercent": //Only include protocols that make up a certain % of the traffic
						if dp.Value.(int64) >= valueCutoff[dp.Timestamp] { //It's in the top percent so log it as-is
							*md = append(*md, dp)
							metricsKept++
						} else {
							otherValues[dp.Timestamp] += dp.Value.(int64)
							metricsDropped++
						}
					case "namedprotocols": //Only include protocols that have an actual name (SSL443 excepted)
						if strings.Index(dp.Tags["proto"], "tcp") != 0 && strings.Index(dp.Tags["proto"], "udp") != 0 && (strings.Index(dp.Tags["proto"], "SSL") != 0 || dp.Tags["proto"] == "SSL443") { //The first characters are not tcp or udp.
							*md = append(*md, dp)
							metricsKept++
						} else {
							otherValues[dp.Timestamp] += dp.Value.(int64)
							metricsDropped++
						}
					case "none": //Log everything. Is OK for viewing short timespans, but calculating, 2,000+ protocols over a multi-day window is bad for Bosun's performance
						*md = append(*md, dp)
						metricsKept++
					}

				}
				//Take the consolidated values and add them now too
				for k, v := range otherValues {
					*md = append(*md, &opentsdb.DataPoint{
						Metric:    xhMetricName,
						Timestamp: k,
						Tags:      opentsdb.TagSet{"vlan": fmt.Sprintf("%d", vlan.VlanId), "proto": extraHopOtherProtoName, "host": c.APIHost, "network": net.Name},
						Value:     v,
					})
				}
			}
		}
	}
	return nil
}

func extraHopGetCertificates(c *gohop.Client, md *opentsdb.MultiDataPoint) error {
	if extraHopCertificateMatch == nil {
		return nil
	}

	if err := extraHopGetCertificateByCount(c, md); err != nil {
		return err
	}

	if err := extraHopGetCertificateByExpiry(c, md); err != nil {
		return err
	}

	return nil
}

func extraHopGetCertificateByCount(c *gohop.Client, md *opentsdb.MultiDataPoint) error {
	//These are the metrics we are populating in this part of the collector
	metricNameCount := "extrahop.certificates"

	//Metadata for the above metrics
	metadata.AddMeta(metricNameCount, nil, "rate", metadata.Gauge, false)
	metadata.AddMeta(metricNameCount, nil, "unit", metadata.Count, false)
	metadata.AddMeta(metricNameCount, nil, "desc", "The number of times a given certificate was seen", false)

	ms := []gohop.MetricSpec{ //Build a metric spec to tell ExtraHop what we want to pull out.
		{Name: "cert_subject", KeyPair: gohop.KeyPair{Key1Regex: "", Key2Regex: "", OpenTSDBKey1: "", Key2OpenTSDBKey2: ""}, OpenTSDBMetric: metricNameCount},
	}
	mrk, err := c.KeyedMetricQuery(gohop.Cycle30Sec, "ssl_server_detail", "activity_group", -60000, 0, ms, []int64{int64(extraHopCertificateActivityGroup)})
	if err != nil {
		return err
	}

	//At this time we have a keyed metric response from ExtraHop. We need to find all the stats, then the values of the stats, and then
	//filter out to only the records we want.

	//This is our function that is going to be executed on each data point in the extrahop dataset
	appendCountPoints := func(c *gohop.Client, md *opentsdb.MultiDataPoint, a *gohop.MetricStatKeyed, b *[]gohop.MetricStatKeyedValue, d *gohop.MetricStatKeyedValue) {
		thisPoint := getSSLDataPointFromSet(metricNameCount, c.APIUrl.Host, a.Time, d)
		if thisPoint != nil {
			*md = append(*md, thisPoint)
		}
	}

	processGohopStat(&mrk, c, md, appendCountPoints) //This will loop through our datapoint structure and execute appendCountPoints on each final data piece

	return nil
}
func extraHopGetCertificateByExpiry(c *gohop.Client, md *opentsdb.MultiDataPoint) error {
	//These are the metrics we are populating in this part of the collector
	metricNameExpiry := "extrahop.certificates.expiry"
	metricNameTillExpiry := "extrahop.certificates.tillexpiry"

	//Metadata for the above metrics
	metadata.AddMeta(metricNameExpiry, nil, "rate", metadata.Gauge, false)
	metadata.AddMeta(metricNameExpiry, nil, "unit", metadata.Timestamp, false)
	metadata.AddMeta(metricNameExpiry, nil, "desc", "Timestamp of when the certificate expires", false)

	metadata.AddMeta(metricNameTillExpiry, nil, "rate", metadata.Gauge, false)
	metadata.AddMeta(metricNameTillExpiry, nil, "unit", metadata.Second, false)
	metadata.AddMeta(metricNameTillExpiry, nil, "desc", "Number of seconds until the certificate expires", false)

	ms := []gohop.MetricSpec{ //Build a metric spec to tell ExtraHop what we want to pull out.
		{Name: "cert_expiration", KeyPair: gohop.KeyPair{Key1Regex: "", Key2Regex: "", OpenTSDBKey1: "", Key2OpenTSDBKey2: ""}, OpenTSDBMetric: metricNameExpiry},
	}
	mrk, err := c.KeyedMetricQuery(gohop.Cycle30Sec, "ssl_server_detail", "activity_group", -60000, 0, ms, []int64{int64(extraHopCertificateActivityGroup)})
	if err != nil {
		return err
	}

	//At this time we have a keyed metric response from ExtraHop. We need to find all the stats, then the values of the stats, and then
	//filter out to only the records we want.

	//This is our function that is going to be executed on each data point in the extrahop dataset
	appendExpiryPoints := func(c *gohop.Client, md *opentsdb.MultiDataPoint, a *gohop.MetricStatKeyed, b *[]gohop.MetricStatKeyedValue, d *gohop.MetricStatKeyedValue) {
		thisPointExpiry := getSSLDataPointFromSet(metricNameExpiry, c.APIUrl.Host, a.Time, d)
		if thisPointExpiry != nil {
			*md = append(*md, thisPointExpiry)
		}

		thisPointTillExpiry := getSSLDataPointFromSet(metricNameTillExpiry, c.APIUrl.Host, a.Time, d)
		if thisPointTillExpiry != nil {
			thisPointTillExpiry.Value = thisPointTillExpiry.Value.(int64) - (a.Time / 1000)
			*md = append(*md, thisPointTillExpiry)
		}
	}

	processGohopStat(&mrk, c, md, appendExpiryPoints) //This will loop through our datapoint structure and execute appendExpiryPoints on each final data piece
	return nil
}

type processFunc func(*gohop.Client, *opentsdb.MultiDataPoint, *gohop.MetricStatKeyed, *[]gohop.MetricStatKeyedValue, *gohop.MetricStatKeyedValue)

func processGohopStat(mrk *gohop.MetricResponseKeyed, c *gohop.Client, md *opentsdb.MultiDataPoint, pc processFunc) {
	for _, a := range mrk.Stats {
		for _, b := range a.Values {
			for _, d := range b {
				pc(c, md, &a, &b, &d)
			}
		}
	}
}

func getSSLDataPointFromSet(metricName, APIUrlHost string, timestamp int64, d *gohop.MetricStatKeyedValue) *opentsdb.DataPoint {
	//The metric key comes as subject:crypt_strength, e.g. *.example.com:RSA_2048
	if strings.IndexAny(d.Key.Str, ":") == -1 { //If the certificate key doesn't contain a : then ignore
		return nil
	}
	certParts := strings.Split(d.Key.Str, ":") //Get the subject and the crypt_strength into seperate parts
	if len(certParts) != 2 {                   //If we don't get exactly 2 parts when we split on the :, then ignore
		return nil
	}
	certSubject := strings.ToLower(certParts[0])            //Make the subject consistently lowercase
	certStrength := certParts[1]                            //Get the crypt_strength
	if !extraHopCertificateMatch.MatchString(certSubject) { //If this certificate does not match the subject name we're filtering on, then ignore
		return nil
	}
	certSubject = strings.Replace(certSubject, "*.", "wild_", -1)                                                     //* is an important part of the subject, but an invalid tag. This should make it pretty obvious that we mean a wildcard cert, not a subdomain of "wild"
	certTags := opentsdb.TagSet{"host": strings.ToLower(APIUrlHost), "subject": certSubject, "keysize": certStrength} //Tags for the metrics
	//Add a key that is the raw expiry time
	return &opentsdb.DataPoint{
		Metric:    metricName,
		Timestamp: timestamp,
		Value:     d.Value,
		Tags:      certTags,
	}
}

//These are used when looping through which L7 traffic to get. We want byte counts and packet counts, and this is the metadata that goes with them.
var l7types = map[string]L7Stats{
	"bytes": {Rate: metadata.Gauge, Unit: metadata.Bytes, Description: "The number of bytes transmitted on this network.You can drill down by server, network, vlan and protocol for further investigations."},
	"pkts":  {Rate: metadata.Gauge, Unit: metadata.Counter, Description: "The number of packets transmitted on this network. You can drill down by server, network, vlan and protocol for further investigations."},
}

// L7Stats describes layer 7 stats to collect
type L7Stats struct {
	Rate        metadata.RateType
	Unit        metadata.Unit
	Description string
}

//Given the % value in the configuration file, calculate what the actual minimum value is for each of the time points returned by ExtraHop
func calculateDataCutoff(k gohop.MetricResponseKeyed) map[int64]int64 {
	sums := make(map[int64]int64)
	rets := make(map[int64]int64)
	for _, dp := range k.Stats {
		for _, dv := range dp.Values {
			for _, dw := range dv {
				sums[dp.Time/1000] += dw.Value
			}

		}
	}
	for k, v := range sums {
		rets[k] = int64(float64(v) * (1 - float64(extraHopTopProtoPerc)/100))
	}
	return rets
}

func ehItemNameToTagSet(c *gohop.Client, ehName string) opentsdb.TagSet {
	thisTagSet := opentsdb.TagSet{"host": strings.ToLower(c.APIUrl.Host)}
	if strings.IndexAny(ehName, ",") == 0 {
		return thisTagSet
	}
	nameParts := strings.Split(ehName, ",")
	for _, p := range nameParts {
		tagParts := strings.Split(p, "=")
		if len(tagParts) > 0 {
			thisTagSet[tagParts[0]] = tagParts[1]
		}
	}
	return thisTagSet
}

func ehMetricNameEscape(metricName string) string {
	metricName = strings.ToLower(metricName)
	metricName = strings.Replace(metricName, " ", "_", -1)
	return fmt.Sprintf("extrahop.application.%v", metricName)
}
