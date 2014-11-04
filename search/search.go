package search

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/scollector/opentsdb"
)

// Search is a struct to hold indexed data about OpenTSDB metric and tag data.
// It is suited to answering questions about: available metrics for a tag set,
// available tag keys for a metric, and available tag values for a metric and
// tag key.
type Search struct {
	// tagk + tagv -> metrics
	Metric qmap
	// metric -> tag keys
	Tagk smap
	// metric + tagk -> tag values
	Tagv qmap
	// Each Record
	MetricTags mtsmap

	sync.RWMutex
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
		Metric:     make(qmap),
		Tagk:       make(smap),
		Tagv:       make(qmap),
		MetricTags: make(mtsmap),
	}
	return &s
}

func (s *Search) Index(mdp opentsdb.MultiDataPoint) {
	s.Lock()
	for _, dp := range mdp {
		var mts MetricTagSet
		mts.Metric = dp.Metric
		mts.Tags = dp.Tags
		s.MetricTags[mts.key()] = mts
		var q duple
		for k, v := range dp.Tags {
			q.A, q.B = k, v
			if _, ok := s.Metric[q]; !ok {
				s.Metric[q] = make(present)
			}
			s.Metric[q][dp.Metric] = struct{}{}

			if _, ok := s.Tagk[dp.Metric]; !ok {
				s.Tagk[dp.Metric] = make(present)
			}
			s.Tagk[dp.Metric][k] = struct{}{}

			q.A, q.B = dp.Metric, k
			if _, ok := s.Tagv[q]; !ok {
				s.Tagv[q] = make(present)
			}
			s.Tagv[q][v] = struct{}{}
		}
	}
	s.Unlock()
}

// Match returns all matching values against search. search is a regex, except
// that `.` is literal, `*` can be used for `.*`, and the entire string is
// searched (`^` and `&` added to ends of search).
func Match(search string, values []string) ([]string, error) {
	v := strings.Replace(search, ".", `\.`, -1)
	v = strings.Replace(v, "*", ".*", -1)
	v = "^" + v + "$"
	re, err := regexp.Compile(v)
	if err != nil {
		return nil, err
	}
	var nvs []string
	for _, nv := range values {
		if re.MatchString(nv) {
			nvs = append(nvs, nv)
		}
	}
	return nvs, nil
}

func (s *Search) Expand(q *opentsdb.Query) error {
	for k, ov := range q.Tags {
		var nvs []string
		for _, v := range strings.Split(ov, "|") {
			v = strings.TrimSpace(v)
			if v == "*" || !strings.Contains(v, "*") {
				nvs = append(nvs, v)
			} else {
				vs := s.TagValuesByMetricTagKey(q.Metric, k)
				ns, err := Match(v, vs)
				if err != nil {
					return err
				}
				nvs = append(nvs, ns...)
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
	s.RLock()
	defer s.RUnlock()
	metrics := make([]string, len(s.Tagk))
	i := 0
	for k := range s.Tagk {
		metrics[i] = k
		i++
	}
	sort.Strings(metrics)
	return metrics
}

func (s *Search) TagValuesByTagKey(Tagk string) []string {
	um := s.UniqueMetrics()
	s.RLock()
	defer s.RUnlock()
	tagvset := make(map[string]bool)
	for _, Metric := range um {
		for _, Tagv := range s.tagValuesByMetricTagKey(Metric, Tagk) {
			tagvset[Tagv] = true
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

func (s *Search) MetricsByTagPair(Tagk, Tagv string) []string {
	s.RLock()
	defer s.RUnlock()
	r := make([]string, 0)
	for k := range s.Metric[duple{Tagk, Tagv}] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) TagKeysByMetric(Metric string) []string {
	s.RLock()
	defer s.RUnlock()
	r := make([]string, 0)
	for k := range s.Tagk[Metric] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) tagValuesByMetricTagKey(Metric, Tagk string) []string {
	r := make([]string, 0)
	for k := range s.Tagv[duple{Metric, Tagk}] {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func (s *Search) TagValuesByMetricTagKey(Metric, Tagk string) []string {
	s.RLock()
	defer s.RUnlock()
	return s.tagValuesByMetricTagKey(Metric, Tagk)
}

func (s *Search) FilteredTagValuesByMetricTagKey(Metric, Tagk string, tsf map[string]string) []string {
	s.RLock()
	defer s.RUnlock()
	tagvset := make(map[string]bool)
	for _, mts := range s.MetricTags {
		if Metric == mts.Metric {
			match := true
			if Tagv, ok := mts.Tags[Tagk]; ok {
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
					tagvset[Tagv] = true
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
