package web

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/StackExchange/tsaf/search"
)

// A Sorted List of Available Metrics
func UniqueMetrics(w http.ResponseWriter, r *http.Request) {
	values := search.UniqueMetrics()
	b, err := json.Marshal(values)
	if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    	return
    }
	w.Write(b)
}

func TagKeysByMetric(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	keys := search.TagKeysByMetric(metric)
	b, err := json.Marshal(keys)
	if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    	return
    }
	w.Write(b)
}

func TagValuesByMetricTagKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagk := vars["tagk"]
	values := search.TagValuesByMetricTagKey(metric, tagk)
	b, err := json.Marshal(values)
	if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    	return
    }
	w.Write(b)
}

func MetricsByTagPair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	values := search.MetricsByTagPair(tagk, tagv)
	b, err := json.Marshal(values)
	if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    	return
    }
	w.Write(b)
}

func TagValuesByTagKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	values := search.TagValuesByTagKey(tagk)
	b, err := json.Marshal(values)
	if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    	return
    }
	w.Write(b)
}