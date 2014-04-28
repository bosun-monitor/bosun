package search

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"

	"github.com/StackExchange/tsaf/third_party/github.com/StackExchange/scollector/opentsdb"
)

/* Questions we want to ask:
1) what metrics are available for a tag set?
2) what tag keys are available for a metric?
3) what tag values are available for a metric+tag key query?
*/

type MetricTagSet struct {
	Metric string          `json:"metric"`
	Tags   opentsdb.TagSet `json:"tags"`
}

func (mts *MetricTagSet) key() string {
	s := make([]string, len(mts.Tags))
	i := 0
	for k, v := range mts.Tags {
		s[i] = fmt.Sprintf("%v=%v,", k, v)
		i++
	}
	sort.Strings(s)
	s = append(s, fmt.Sprintf("metric=%v", mts.Metric))
	return strings.Join(s, "")
}

type QMap map[Query]Present
type SMap map[string]Present
type MTSMap map[string]MetricTagSet

var (
	// tagk + tagv -> metrics
	Metric = make(QMap)
	// metric -> tag keys
	Tagk = make(SMap)
	// metric + tagk -> tag values
	Tagv = make(QMap)
	// Each Record
	MetricTags = make(MTSMap)

	lock = sync.RWMutex{}
)

type Present map[string]interface{}

type Query struct {
	A, B string
}

var (
	dc = make(chan *opentsdb.DataPoint)
)

func init() {
	go Process(dc)
}

// HTTPExtract populates the search indexes with OpenTSDB tags and metrics from
// body. body is a JSON string of an OpenTSDB v2 /api/put request. body may be
// gzipped.
func HTTPExtract(body []byte) {
	if r, err := gzip.NewReader(bytes.NewBuffer(body)); err == nil {
		if b, err := ioutil.ReadAll(r); err == nil {
			body = b
		}
		r.Close()
	}
	var dp opentsdb.DataPoint
	var mdp opentsdb.MultiDataPoint
	var err error
	if err = json.Unmarshal(body, &dp); err == nil {
		mdp = append(mdp, &dp)
	} else if err := json.Unmarshal(body, &mdp); err != nil {
		return
	}
	for _, d := range mdp {
		dc <- d
	}
}

func Process(c chan *opentsdb.DataPoint) {
	for dp := range c {
		go func(dp *opentsdb.DataPoint) {
			lock.Lock()
			defer lock.Unlock()
			var mts MetricTagSet
			mts.Metric = dp.Metric
			mts.Tags = dp.Tags
			MetricTags[mts.key()] = mts
			var q Query
			for k, v := range dp.Tags {
				q.A, q.B = k, v
				if _, ok := Metric[q]; !ok {
					Metric[q] = make(Present)
				}
				Metric[q][dp.Metric] = nil

				if _, ok := Tagk[dp.Metric]; !ok {
					Tagk[dp.Metric] = make(Present)
				}
				Tagk[dp.Metric][k] = nil

				q.A, q.B = dp.Metric, k
				if _, ok := Tagv[q]; !ok {
					Tagv[q] = make(Present)
				}
				Tagv[q][v] = nil
			}
		}(dp)
	}
}

func UniqueMetrics() []string {
	lock.RLock()
	defer lock.RUnlock()
	metrics := make([]string, len(Tagk))
	i := 0
	for k := range Tagk {
		metrics[i] = k
		i++
	}
	sort.Strings(metrics)
	return metrics
}

func TagValuesByTagKey(tagk string) []string {
	lock.RLock()
	defer lock.RUnlock()
	tagvset := make(map[string]bool)
	for _, metric := range UniqueMetrics() {
		for _, tagv := range TagValuesByMetricTagKey(metric, tagk) {
			tagvset[tagv] = true
		}
	}
	tagvs := make([]string, len(tagvset))
	i := 0
	for k := range tagvset {
		tagvs[i] = k
		i++
	}
	sort.Strings(tagvs)
	return tagvs

}

func MetricsByTagPair(tagk, tagv string) []string {
	lock.RLock()
	defer lock.RUnlock()
	r := make([]string, 0)
	for k := range Metric[Query{tagk, tagv}] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func TagKeysByMetric(metric string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range Tagk[metric] {
		r = append(r, k)
	}
	return r
}

func TagValuesByMetricTagKey(metric, tagk string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range Tagv[Query{metric, tagk}] {
		r = append(r, k)
	}
	return r
}

func FilteredTagValuesByMetricTagKey(metric, tagk string, tsf map[string]string) []string {
	lock.RLock()
	defer lock.RUnlock()
	tagvset := make(map[string]bool)
	for _, mts := range MetricTags {
		if metric == mts.Metric {
			match := true
			if tagv, ok := mts.Tags[tagk]; ok {
				for tpk, tpv := range tsf {
					if v, ok := mts.Tags[tpk]; ok {
						if !(v == tpv) {
							match = false
						}
					} else {
						match = false
					}
				}
				if match {
					tagvset[tagv] = true
				}
			}
		}
	}
	tagvs := make([]string, len(tagvset))
	i := 0
	for k := range tagvset {
		tagvs[i] = k
		i++
	}
	sort.Strings(tagvs)
	return tagvs
}
