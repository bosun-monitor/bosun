package sched

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/collect"
)

// Search is a struct to hold indexed data about OpenTSDB metric and tag data.
// It is suited to answering questions about: available metrics for a tag set,
// available tag keys for a metric, and available tag values for a metric and
// tag key.
type Search struct {
	// tagk + tagv -> metrics
	metric qmap
	// metric -> tag keys
	tagk smap
	// metric + tagk -> tag values
	tagv qmap
	// Each Record
	metricTags mtsmap

	lock sync.RWMutex
	ch   chan opentsdb.MultiDataPoint
}

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

type qmap map[duple]present
type smap map[string]present
type mtsmap map[string]MetricTagSet
type present map[string]struct{}

type duple struct {
	A, B string
}

func NewSearch() *Search {
	s := Search{
		metric:     make(qmap),
		tagk:       make(smap),
		tagv:       make(qmap),
		metricTags: make(mtsmap),
		ch:         make(chan opentsdb.MultiDataPoint, 1000),
	}
	go s.Process()
	collect.Set("search.ch_len", nil, func() interface{} {
		return len(s.ch)
	})
	return &s
}

func (s *Search) Process() {
	for mdp := range s.ch {
		s.lock.Lock()
		for _, dp := range mdp {
			var mts MetricTagSet
			mts.Metric = dp.Metric
			mts.Tags = dp.Tags
			s.metricTags[mts.key()] = mts
			var q duple
			for k, v := range dp.Tags {
				q.A, q.B = k, v
				if _, ok := s.metric[q]; !ok {
					s.metric[q] = make(present)
				}
				s.metric[q][dp.Metric] = struct{}{}

				if _, ok := s.tagk[dp.Metric]; !ok {
					s.tagk[dp.Metric] = make(present)
				}
				s.tagk[dp.Metric][k] = struct{}{}

				q.A, q.B = dp.Metric, k
				if _, ok := s.tagv[q]; !ok {
					s.tagv[q] = make(present)
				}
				s.tagv[q][v] = struct{}{}
			}
		}
		s.lock.Unlock()
	}
}

func (s *Search) Index(mdp opentsdb.MultiDataPoint) {
	select {
	case s.ch <- mdp:
	default:
		collect.Add("search.batch_drop", nil, 1)
	}
}

func (s *Search) Expand(q *opentsdb.Query) error {
	for k, ov := range q.Tags {
		v := ov
		if v == "*" || !strings.Contains(v, "*") || strings.Contains(v, "|") {
			continue
		}
		v = strings.Replace(v, ".", `\.`, -1)
		v = strings.Replace(v, "*", ".*", -1)
		v = "^" + v + "$"
		re := regexp.MustCompile(v)
		var nvs []string
		vs := s.TagValuesByMetricTagKey(q.Metric, k)
		for _, nv := range vs {
			if re.MatchString(nv) {
				nvs = append(nvs, nv)
			}
		}
		if len(nvs) == 0 {
			return fmt.Errorf("expr: no tags matching %s=%s", k, ov)
		}
		q.Tags[k] = strings.Join(nvs, "|")
	}
	return nil
}

func (s *Search) UniqueMetrics() []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	metrics := make([]string, len(s.tagk))
	i := 0
	for k := range s.tagk {
		metrics[i] = k
		i++
	}
	sort.Strings(metrics)
	return metrics
}

func (s *Search) TagValuesByTagKey(tagk string) []string {
	um := s.UniqueMetrics()
	s.lock.RLock()
	defer s.lock.RUnlock()
	tagvset := make(map[string]bool)
	for _, metric := range um {
		for _, tagv := range s.tagValuesByMetricTagKey(metric, tagk) {
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

func (s *Search) MetricsByTagPair(tagk, tagv string) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	r := make([]string, 0)
	for k := range s.metric[duple{tagk, tagv}] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) TagKeysByMetric(metric string) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	r := make([]string, 0)
	for k := range s.tagk[metric] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) tagValuesByMetricTagKey(metric, tagk string) []string {
	r := make([]string, 0)
	for k := range s.tagv[duple{metric, tagk}] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) TagValuesByMetricTagKey(metric, tagk string) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.tagValuesByMetricTagKey(metric, tagk)
}

func (s *Search) FilteredTagValuesByMetricTagKey(metric, tagk string, tsf map[string]string) []string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	tagvset := make(map[string]bool)
	for _, mts := range s.metricTags {
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
