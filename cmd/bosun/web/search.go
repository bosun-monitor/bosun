package web

import (
	"net/http"
	"strings"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/gorilla/mux"
)

// A Sorted List of Available Metrics
func UniqueMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := schedule.Search.UniqueMetrics()
	return values, nil
}

func TagKeysByMetric(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	keys := schedule.Search.TagKeysByMetric(metric)
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
		values = schedule.Search.FilteredTagValuesByMetricTagKey(metric, tagk, tsf)
	} else {
		values = schedule.Search.TagValuesByMetricTagKey(metric, tagk)
	}
	return values, nil
}

func MetricsByTagPair(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	values := schedule.Search.MetricsByTagPair(tagk, tagv)
	return values, nil
}

func TagValuesByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	values := schedule.Search.TagValuesByTagKey(tagk)
	return values, nil
}
