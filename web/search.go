package web

import (
	"net/http"
	"strings"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/gorilla/mux"
	"github.com/StackExchange/bosun/search"
)

// A Sorted List of Available Metrics
func UniqueMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := search.UniqueMetrics()
	return values, nil
}

func TagKeysByMetric(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	keys := search.TagKeysByMetric(metric)
	return keys, nil
}

func TagValuesByMetricTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagk := vars["tagk"]
	q := r.URL.Query()
	var values []string
	if len(q) > 0 {
		tsf := make(map[string]string)
		for k, v := range q {
			tsf[k] = strings.Join(v, "")
		}
		values = search.FilteredTagValuesByMetricTagKey(metric, tagk, tsf)
	} else {
		values = search.TagValuesByMetricTagKey(metric, tagk)
	}
	return values, nil
}

func MetricsByTagPair(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	values := search.MetricsByTagPair(tagk, tagv)
	return values, nil
}

func TagValuesByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	values := search.TagValuesByTagKey(tagk)
	return values, nil
}
