package collectors

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/kylebrandt/gohop"
)

const extraHopIntervalSeconds int = 30

var extraHopFilterProtoBy string  //What to filter the traffic by. Valid values are "namedprotocols", "toppercent" or "none"
var extraHopTopProtoPerc int      //Only log the top % of protocols by volume
var extraHopOtherProtoName string //What name to log the "other" data under.
var extraHopL7Description string  //What to append to the end of the L7 description metadata to explain what is and isn't filtered out

//Register a collector for ExtraHop
func ExtraHop(host, apikey, filterby string, filterpercent int) error {
	if host == "" || apikey == "" {
		return fmt.Errorf("Empty host or API key for ExtraHop.")
	}
	extraHopFilterProtoBy = filterby
	switch filterby { //Set up options
	case "toppercent":
		extraHopL7Description = fmt.Sprintf("Only the top %d percent of traffic has its protocols logged, the remainder is tagged as as proto=otherprotos", extraHopTopProtoPerc)
		extraHopOtherProtoName = "otherprotos"
		if filterpercent > 0 && filterpercent < 100 {
			extraHopTopProtoPerc = filterpercent
		} else {
			return fmt.Errorf("Invalid ExtraHop FilterPercent value (%d). Number should be between 1 and 99.", filterpercent)
		}
	case "namedprotocols":
		extraHopL7Description = "Only named protocols are logged. Any unnamed protocol (A protocol name starting with tcp, udp or ssl) is tagged as proto=unnamed"
		extraHopOtherProtoName = "unnamed"
	//There is also case "none", but in that case the options we need to keep as default, so there's actually nothing to do here.
	default:
		return fmt.Errorf("Invalid ExtraHop FilterBy option (%s). Valid options are namedprotocols, toppercent or none.", filterby)

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
	return md, nil
}

/*
	This grabs the complex metrics of the L7 traffic from ExtraHop. It is a complex type because the data is not just a simple time series,
	the data needs to be tagged with vlan, protocol, etc. We can do the network and vlan tagging ourselves, but the protocol tagging comes
	from ExtraHop itself.
*/
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

//These are used when looping through which L7 traffic to get. We want byte counts and packet counts, and this is the metadata that goes with them.
var l7types = map[string]L7Stats{
	"bytes": {Rate: metadata.Gauge, Unit: metadata.Bytes, Description: "The number of bytes transmitted on this network.You can drill down by server, network, vlan and protocol for further investigations."},
	"pkts":  {Rate: metadata.Gauge, Unit: metadata.Counter, Description: "The number of packets transmitted on this network. You can drill down by server, network, vlan and protocol for further investigations."},
}

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
