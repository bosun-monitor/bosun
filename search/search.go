package search

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/StackExchange/tsaf/opentsdb"
)

/* Questions we want to ask:
1) what metrics are available for a tag set?
2) what tag keys are available for a metric?
3) what tag values are available for a metric+tag key query?
*/

var (
	// tagk + tagv -> metrics
	metric = make(map[Query]Present)
	// metric -> tag keys
	tagk = make(map[string]Present)
	// metric + tagk -> tag values
	tagv = make(map[Query]Present)

	lock = sync.RWMutex{}
)

type Present map[string]interface{}

type Query struct {
	A, B string
}

func Extract(c chan *opentsdb.DataPoint) func(*http.Request, []byte) error {
	return func(r *http.Request, body []byte) error {
		var dp opentsdb.DataPoint
		var mdp opentsdb.MultiDataPoint
		var err error
		if err = json.Unmarshal(body, &dp); err == nil {
			mdp = append(mdp, &dp)
		} else if err := json.Unmarshal(body, &mdp); err != nil {
			return err
		}
		for _, d := range mdp {
			c <- d
		}
		return nil
	}
}

func Process(c chan *opentsdb.DataPoint) {
	for dp := range c {
		go func(dp *opentsdb.DataPoint) {
			log.Println("proc", dp)
			lock.Lock()
			defer lock.Unlock()
			var q Query
			for k, v := range dp.Tags {
				q.A, q.B = k, v
				if _, ok := metric[q]; !ok {
					metric[q] = make(Present)
				}
				metric[q][dp.Metric] = nil

				if _, ok := tagk[dp.Metric]; !ok {
					tagk[dp.Metric] = make(Present)
				}
				tagk[dp.Metric][k] = nil

				q.A, q.B = dp.Metric, k
				if _, ok := tagv[q]; !ok {
					tagv[q] = make(Present)
				}
				tagv[q][v] = nil
			}
		}(dp)
	}
}

func Metrics(tagk, tagv string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range metric[Query{tagk, tagv}] {
		r = append(r, k)
	}
	return r
}

func TagKeys(metric string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range tagk[metric] {
		r = append(r, k)
	}
	return r
}

func TagValues(metric, tagk string) []string {
	lock.RLock()
	defer lock.RUnlock()
	var r []string
	for k := range tagv[Query{metric, tagk}] {
		r = append(r, k)
	}
	return r
}
