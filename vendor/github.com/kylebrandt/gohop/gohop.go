package gohop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"bosun.org/opentsdb"
)

type Client struct {
	APIKey  string
	APIUrl  *url.URL
	APIHost string
}

// NewClient creates an instance of a ExtraHop REST API v1 client.
func NewClient(APIUrl, APIKey string) *Client {
	u, err := url.Parse(APIUrl)

	if err != nil {
		log.Fatal(err)
	}

	return &Client{
		APIKey:  APIKey,
		APIUrl:  u,
		APIHost: u.Host,
	}
}

func (c *Client) request(path, method string, data interface{}, dst interface{}) error {
	url := fmt.Sprintf("%s/api/v1/%s", c.APIUrl, path)
	var d []byte
	var err error
	if data != nil {
		d, err = json.Marshal(&data)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(d))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("ExtraHop apikey=%s", c.APIKey))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if dst == nil {
		return nil
	}
	if resp.StatusCode != 200 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("Response Code %v: %s", resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (c *Client) post(path string, data interface{}, dst interface{}) error {
	return c.request(path, "POST", data, dst)
}

func (c *Client) get(path string, data interface{}, dst interface{}) error {
	return c.request(path, "GET", data, dst)
}

// Posible Values for the Cycle Parameter of a Metric Query
var (
	CycleAuto  = "auto"
	Cycle30Sec = "30sec"
	Cycle5Min  = "5min"
	Cycle1Hr   = "1hr"
	Cycle24Hr  = "24hr"
)

//Metrics
type MetricQuery struct {
	// Can be"auto", "30sec", "5min", "1hr", "24hr"
	Cycle string `json:"cycle:`
	From  int64  `json:"from"`
	// Currently these seem secret, can be net or app though
	Category string `json:"metric_category"`
	//Stats.Values in the response becomes increases in length when there are more than one stat MetricSpec Requested
	Specs []MetricSpec `json:"metric_specs"`
	// OID becomes
	ObjectIds []int64 `json:"object_ids"`
	//Can be "network", "device", "application", "vlan", "device_group", and "activity_group".
	Type  string `json:"object_type"`
	Until int64  `json:"until"`
}

type KeyPair struct {
	Key1Regex        string `json:"key1,omitempty"`
	Key2Regex        string `json:"key2,omitempty"` // I can't find an example using 2 keys at the moment
	OpenTSDBKey1     string `json:"-"`
	Key2OpenTSDBKey2 string `json:"-"`
}

type MetricSpec struct {
	Name     string `json:"name"`
	CalcType string `json:"calc_type"`
	// The type of Stats.Values changes when there are keys added. It goes from []ints to an [][]structs, so the tag can be included
	KeyPair
	Percentiles []int64 `json:"percentiles,omitempty"`
	// The following are not part of the extrahop API
	OpenTSDBMetric string `json:"-"`
}

type MetricStat struct {
	Duration int64 `json:"duration"`
	Oid      int64 `json:"oid"`
	Time     int64 `json:"time"`
}

type MetricStatSimple struct {
	MetricStat
	Values []int64 `json:"values"`
}

type MetricStatKeyed struct {
	MetricStat
	Values [][]struct {
		Key struct {
			KeyType string `json:"key_type"`
			Str     string `json:"str"`
		} `json:"key"`
		Value int64  `json:"value"`
		Vtype string `json:"vtype"`
	} `json:"values"`
}

type MetricResponseBase struct {
	Cycle  string `json:"cycle"`
	From   int64  `json:"from"`
	NodeID int64  `json:"node_id"`
	Until  int64  `json:"until"`
}

type MetricResponseSimple struct {
	MetricResponseBase
	Stats []MetricStatSimple `json:"stats"`
}

type MetricResponseKeyed struct {
	MetricResponseBase
	Stats []MetricStatKeyed `json:"stats"`
}

func (mr *MetricResponseSimple) OpenTSDBDataPoints(metricNames []string, objectKey string, objectIdToName map[int64]string) (opentsdb.MultiDataPoint, error) {
	// Each position in Values should corespond to the order of
	// of Specs object. So len of Values == len(mq.Specs) I think.
	// Each item in Stats has a UID, which will map to the
	// requested object IDs
	var md opentsdb.MultiDataPoint
	for _, s := range mr.Stats {
		name, ok := objectIdToName[s.Oid]
		var tagSet opentsdb.TagSet
		if objectKey != "" && name != "" {
			tagSet = opentsdb.TagSet{objectKey: name}
		} else {
			tagSet = nil
		}
		if !ok {
			return md, fmt.Errorf("no name found for oid %s", s.Oid)
		}
		time := s.Time
		if time < 1 {
			return md, fmt.Errorf("encountered a time less than 1")
		}
		for i, v := range s.Values {
			if len(metricNames) < i {
				return md, fmt.Errorf("no corresponding metric name at index %v", i)
			}
			metricName := metricNames[i]
			md = append(md, &opentsdb.DataPoint{
				Metric:    metricName,
				Timestamp: time / 1000,
				Tags:      tagSet,
				Value:     v,
			})
		}
	}
	return md, nil
}

// Simple Metric query is for when you are making a query that doesn't
// have any facets ("Keys").
func (c *Client) SimpleMetricQuery(cycle, category, objectType string, fromMS, untilMS int64, metricsNames []string, objectIds []int64) (MetricResponseSimple, error) {
	mq := MetricQuery{
		Cycle:     cycle,
		Category:  category,
		ObjectIds: objectIds,
		Type:      objectType,
		From:      fromMS,
		Until:     untilMS,
	}
	for _, name := range metricsNames {
		mq.Specs = append(mq.Specs, MetricSpec{Name: name})
	}
	m := MetricResponseSimple{}
	err := c.post("metrics", &mq, &m)
	return m, err
}

// Keyed Metric query is for when you are making a query that has facets ("Keys"). For example bytes "By L7 Protocol"
func (mr *MetricResponseKeyed) OpenTSDBDataPoints(metrics []MetricSpec, objectKey string, objectIdToName map[int64]string) (opentsdb.MultiDataPoint, error) {
	// Only tested against one key, didn't find example with 2 keys yet
	var md opentsdb.MultiDataPoint
	for _, s := range mr.Stats {
		name, ok := objectIdToName[s.Oid]
		if !ok {
			return md, fmt.Errorf("no name found for oid %s", s.Oid)
		}
		time := s.Time
		if time < 1 {
			return md, fmt.Errorf("encountered a time less than 1")
		}
		for i, values := range s.Values {
			if len(metrics) < i {
				return md, fmt.Errorf("no corresponding metric name at index %v", i)
			}
			metricName := metrics[i].OpenTSDBMetric
			key1 := metrics[i].OpenTSDBKey1
			for _, v := range values {
				md = append(md, &opentsdb.DataPoint{
					Metric:    metricName,
					Timestamp: time / 1000,
					Tags:      opentsdb.TagSet{objectKey: name, key1: v.Key.Str},
					Value:     v.Value,
				})
			}
		}
	}
	return md, nil
}

func (c *Client) KeyedMetricQuery(cycle, category, objectType string, fromMS, untilMS int64, metrics []MetricSpec,
	objectIds []int64) (MetricResponseKeyed, error) {
	mq := MetricQuery{
		Cycle:     cycle,
		Category:  category,
		ObjectIds: objectIds,
		Type:      objectType,
		From:      fromMS,
		Until:     untilMS,
	}
	for _, spec := range metrics {
		mq.Specs = append(mq.Specs, spec)
	}
	m := MetricResponseKeyed{}
	err := c.post("metrics", &mq, &m)
	return m, err
}

type NetworkList []struct {
	Id          int64    `json:"id"`
	NodeId      int64    `json:"node_id"`
	Description string   `json:"description"`
	Name        string   `json:"name"`
	Idle        bool     `json:"idle"`
	Vlans       VlanList //This is not part of the JSON returned by ExtraHop's /network endpoint, so this gets populated by a 2nd step.
}

type VlanList []struct {
	Id          int64  `json:"id"`
	NetworkId   int64  `json:"network_id"`
	VlanId      int64  `json:"vlanid"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *Client) GetNetworkList(FetchVlans bool) (NetworkList, error) {
	l := NetworkList{}
	err := c.get("networks", "", &l)
	if err != nil {
		return nil, err
	}
	if FetchVlans {
		for k, dp := range l {
			err := c.GetVlanList(dp.Id, &dp.Vlans)
			if err == nil {
				l[k] = dp
			}
		}
	}
	return l, err
}

func (c *Client) GetVlanList(NetworkId int64, l *VlanList) error {
	url := fmt.Sprintf("networks/%d/vlans", NetworkId)
	err := c.get(url, "", &l)
	return err
}

type ExtraHopMetric struct {
	ObjectType         string
	ObjectId           int64
	MetricCategory     string
	MetricSpecName     string
	MetricSpecCalcType string
}

func StoEHMetric(i string) (ExtraHopMetric, error) {
	v := strings.Split(i, ".")

	if len(v) < 4 || len(v) > 5 {
		return ExtraHopMetric{}, errors.New(fmt.Sprintf("Provided metric (%s) had %d parts. Metric must have either 4 or 5 parts", i, len(v)))
	}

	if len(v) == 4 {
		v = append(v, "")
	}

	oid, err := strconv.Atoi(v[1])

	if err != nil {
		return ExtraHopMetric{}, errors.New(fmt.Sprintf("Provided metric (%s) does not have a number as its second part (%s) ", i, v[1]))
	}

	return ExtraHopMetric{
		ObjectType:         v[0],
		ObjectId:           int64(oid),
		MetricCategory:     v[2],
		MetricSpecName:     v[3],
		MetricSpecCalcType: v[4],
	}, nil

}
