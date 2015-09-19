package web

import (
	"net/http"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/gorilla/mux"
	"bosun.org/opentsdb"
)

// UniqueMetrics returns a sorted list of available metrics.
func UniqueMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := schedule.Search.UniqueMetrics()
	// remove anything starting with double underscore.
	q := r.URL.Query()
	if v := q.Get("unfiltered"); v != "" {
		return values, nil
	}
	filtered := []string{}
	for _, v := range values {
		if len(v) < 2 || v[0:2] != "__" {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
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
	return schedule.Search.TagValuesByMetricTagKey(metric, tagk, 0), nil
}

func MetricsByTagPair(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	values := schedule.Search.MetricsByTagPair(tagk, tagv)
	return values, nil
}

func MetricsWithTagKeys(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := schedule.Search.MetricsWithTagKeys()
	return values, nil
}

func TagValuesByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	s := r.FormValue("since")
	var since opentsdb.Duration
	if s == "default" {
		since = schedule.Conf.SearchSince
	} else if s != "" {
		var err error
		since, err = opentsdb.ParseDuration(s)
		if err != nil {
			return nil, err
		}
	}
	values := schedule.Search.TagValuesByTagKey(tagk, time.Duration(since))
	return values, nil
}
