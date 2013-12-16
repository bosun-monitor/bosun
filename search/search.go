package search

import (
	"encoding/json"
	"sync"

	"github.com/StackExchange/tcollector/opentsdb"
)

/* Questions we want to ask:
1) what metrics are available for a tag set?
2) what tag keys are available for a metric?
3) what tag values are available for a metric+tag key query?
*/

type QMap map[Query]Present
type SMap map[string]Present

var (
	// tagk + tagv -> metrics
	Metric = make(QMap)
	// metric -> tag keys
	Tagk = make(SMap)
	// metric + tagk -> tag values
	Tagv = make(QMap)

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

func HTTPExtract(body []byte) {
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

func Metrics(tagk, tagv string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range Metric[Query{tagk, tagv}] {
		r = append(r, k)
	}
	return r
}

func TagKeys(metric string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range Tagk[metric] {
		r = append(r, k)
	}
	return r
}

func TagValues(metric, tagk string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range Tagv[Query{metric, tagk}] {
		r = append(r, k)
	}
	return r
}
