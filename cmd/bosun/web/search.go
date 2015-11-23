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
	values, err := schedule.Search.UniqueMetrics()
	if err != nil {
		return nil, err
	}
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
	return schedule.Search.TagKeysByMetric(metric)
}

func TagValuesByMetricTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagk := vars["tagk"]
	return schedule.Search.TagValuesByMetricTagKey(metric, tagk, 0)
}

func FilteredTagsetsByMetric(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagset := opentsdb.TagSet{}
	var err error
	ts := r.FormValue("tags")
	if ts != "" {
		if tagset, err = opentsdb.ParseTags(ts); err != nil {
			return nil, err
		}
	}
	return schedule.Search.FilteredTagSets(metric, tagset)
}

func MetricsByTagPair(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	return schedule.Search.MetricsByTagPair(tagk, tagv)
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
	return schedule.Search.TagValuesByTagKey(tagk, time.Duration(since))
}
