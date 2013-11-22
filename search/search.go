package search

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/StackExchange/tsaf/opentsdb"
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

func ExtractTCP() func([]byte) string {
	return func(body []byte) (s string) {
		sp := strings.Split(strings.TrimSpace(string(body)), " ")
		if len(sp) < 4 {
			return
		} else if sp[0] != "put" {
			return
		}
		i, err := strconv.ParseInt(sp[2], 10, 64)
		if err != nil {
			return
		}
		d := opentsdb.DataPoint{
			Metric:    sp[1],
			Timestamp: i,
			Value:     sp[3],
			Tags:      make(map[string]string),
		}
		for _, t := range sp[4:] {
			ts := strings.Split(t, "=")
			if len(ts) != 2 {
				continue
			}
			d.Tags[ts[0]] = ts[1]
		}
		dc <- &d
		return
	}
}

func ExtractHTTP() func(*http.Request, []byte) error {
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
			dc <- d
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
