package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/gorilla/mux"
)

// UniqueMetrics returns a sorted list of available metrics.
func UniqueMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	q := r.URL.Query()
	var epoch int64
	if v := q.Get("since"); v != "" {
		var err error
		epoch, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			serveError(w, fmt.Errorf("could not convert since parameter (expecting epoch value): %v", err))
		}
	}
	values, err := schedule.Search.UniqueMetrics(epoch)
	if err != nil {
		return nil, err
	}
	// remove anything starting with double underscore.
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

func MetricsByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagValues, err := schedule.Search.TagValuesByTagKey(tagk, time.Duration(schedule.Conf.SearchSince))
	if err != nil {
		return nil, err
	}
	// map[tagv][metrics...]
	tagvMetrics := make(map[string][]string)
	for _, tagv := range tagValues {
		metrics, err := schedule.Search.MetricsByTagPair(tagk, tagv)
		if err != nil {
			return nil, err
		}
		tagvMetrics[tagv] = metrics
	}
	return tagvMetrics, nil
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
