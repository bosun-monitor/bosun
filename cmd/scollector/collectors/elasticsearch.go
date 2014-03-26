package collectors

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"

	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_elasticsearch})
}

const (
	esURL = "http://localhost:9200/"
)

var (
	esPreV1     = regexp.MustCompile(`^0\.`)
	esStatusMap = map[string]int{
		"green":  0,
		"yellow": 1,
		"red":    2,
	}
)

func esReq(path, query string, v interface{}) error {
	u := &url.URL{
		Scheme:   "http",
		Host:     "localhost:9200",
		Path:     path,
		RawQuery: query,
	}
	resp, err := http.Get(u.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return json.Unmarshal(b, v)
}

func divInterfaceFlt(a, b interface{}) (float64, error) {
	af, ok := a.(float64)
	if !ok {
		return af, errors.New("couldn't convert to float64")
	}
	bf, ok := b.(float64)
	if !ok {
		return bf, errors.New("couldn't convert to float64")
	}
	return af / bf, nil
}

func c_elasticsearch() opentsdb.MultiDataPoint {
	var status esStatus
	if err := esReq("/", "", &status); err != nil {
		return nil
	}
	var stats esStats
	if err := esReq(esStatsURL(status.Version.Number), "", &stats); err != nil {
		return nil
	}
	var clusterState esClusterState
	if err := esReq("/_cluster/state", "?filter_routing_table=true&filter_metadata=true&filter_blocks=true", &clusterState); err != nil {
		return nil
	}
	var md opentsdb.MultiDataPoint
	add := func(name string, val interface{}, ts opentsdb.TagSet) {
		tags := opentsdb.TagSet{"cluster": stats.ClusterName}
		for k, v := range ts {
			tags[k] = v
		}
		Add(&md, "elastic."+name, val, tags)
	}
	for nodeid, nstats := range stats.Nodes {
		isMaster := nodeid == clusterState.MasterNode
		if isMaster {
			cstats := make(map[string]interface{})
			if err := esReq("/_cluster/health", "", &cstats); err != nil {
				return nil
			}
			for k, v := range cstats {
				switch t := v.(type) {
				case string:
					if k != "status" {
						continue
					}
					var present bool
					if v, present = esStatusMap[t]; !present {
						v = -1
					}
				case float64:
					// break
				default:
					continue
				}
				add("cluster."+k, v, nil)
			}
		}
		for k, v := range nstats.Indices {
			switch k {
			case "docs":
				add("num_docs", v["count"], nil)
			case "store":
				add("indices.size", v["size_in_bytes"], nil)
			case "indexing":
				add("indexing.index_total", v["index_total"], nil)
				add("indexing.index_time", v["index_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["index_time_in_millis"], v["index_total"]); err == nil {
					add("indexing.time_per_index", f, nil)
				}
				add("indexing.index_current", v["index_current"], nil)
				add("indexing.delete_total", v["delete_total"], nil)
				add("indexing.delete_time", v["delete_time_in_millis"], nil)
				add("indexing.delete_current", v["delete_current"], nil)
				if f, err := divInterfaceFlt(v["delete_time_in_millis"], v["delete_total"]); err == nil {
					add("indexing.time_per_delete", f, nil)
				}
			case "get":
				add("get.total", v["total"], nil)
				add("get.time", v["time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["time_in_millis"], v["total"]); err == nil {
					add("get.time_per_get", f, nil)
				}
				add("get.exists_total", v["exists_total"], nil)
				add("get.exists_time", v["exists_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["exists_time_in_millis"], v["exists_total"]); err == nil {
					add("get.time_per_get_exists", f, nil)
				}
				add("get.missing_total", v["missing_total"], nil)
				add("get.missing_time", v["missing_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["missing_time_in_millis"], v["missing_total"]); err == nil {
					add("get.time_per_get_missing", f, nil)
				}
			case "search":
				add("search.query_total", v["query_total"], nil)
				add("search.query_time", v["query_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["query_time_in_millis"], v["query_total"]); err == nil {
					add("search.time_per_query", f, nil)
				}
				add("search.query_current", v["query_current"], nil)
				add("search.fetch_total", v["fetch_total"], nil)
				add("search.fetch_time", v["fetch_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["fetch_time_in_millis"], v["fetch_total"]); err == nil {
					add("search.time_per_fetch", f, nil)
				}
				add("search.fetch_current", v["fetch_current"], nil)
			case "cache":
				add("cache.field.evictions", v["field_evictions"], nil)
				add("cache.field.size", v["field_size_in_bytes"], nil)
				add("cache.filter.count", v["filter_count"], nil)
				add("cache.filter.evictions", v["filter_evictions"], nil)
				add("cache.filter.size", v["filter_size_in_bytes"], nil)
			case "merges":
				add("merges.current", v["current"], nil)
				add("merges.total", v["total"], nil)
				add("merges.total_time", v["total_time_in_millis"], nil)
				if f, err := divInterfaceFlt(v["total_time_in_millis"], v["total"]); err == nil {
					add("merges.time_per_merge", f, nil)
				}
			}
		}
		for k, v := range nstats.Process {
			switch k {
			case "open_file_descriptors": // ES 0.17
				add("process.open_file_descriptors", v, nil)
			case "fd": // ES 0.16
				if t, present := nstats.Process["total"]; present {
					add("process.open_file_descriptors", t, nil)
				}
			case "cpu":
				v := v.(map[string]interface{})
				add("process.cpu.percent", v["percent"], nil)
				add("process.cpu.sys", v["sys_in_millis"].(float64)/1000., nil)
				add("process.cpu.user", v["user_in_millis"].(float64)/1000., nil)
			case "mem":
				v := v.(map[string]interface{})
				add("process.mem.resident", v["resident_in_bytes"], nil)
				add("process.mem.shared", v["share_in_bytes"], nil)
				add("process.mem.total_virtual", v["total_virtual_in_bytes"], nil)
			}
		}
		for k, v := range nstats.JVM {
			switch k {
			case "mem":
				v := v.(map[string]interface{})
				add("jvm.mem.heap_used", v["heap_used_in_bytes"], nil)
				add("jvm.mem.heap_committed", v["heap_committed_in_bytes"], nil)
				add("jvm.mem.non_heap_used", v["non_heap_used_in_bytes"], nil)
				add("jvm.mem.non_heap_committed", v["non_heap_committed_in_bytes"], nil)
			case "threads":
				v := v.(map[string]interface{})
				add("jvm.threads.count", v["count"], nil)
				add("jvm.threads.peak_count", v["peak_count"], nil)
			case "gc":
				v := v.(map[string]interface{})
				c := v["collectors"].(map[string]interface{})
				for k, v := range c {
					v := v.(map[string]interface{})
					ts := opentsdb.TagSet{"gc": k}
					add("jvm.gc.collection_count", v["collection_count"], ts)
					add("jvm.gc.collection_time", v["collection_time_in_millis"].(float64)/1000, ts)
				}
			}
		}
		for k, v := range nstats.Network {
			switch k {
			case "tcp":
				for k, v := range v.(map[string]interface{}) {
					switch v.(type) {
					case float64:
						add("network.tcp."+k, v, nil)
					}
				}
			}
		}
		for k, v := range nstats.Transport {
			switch v.(type) {
			case float64:
				add("transport."+k, v, nil)
			}
		}
		for k, v := range nstats.HTTP {
			switch v.(type) {
			case float64:
				add("http."+k, v, nil)
			}
		}
	}
	return md
}

func esStatsURL(version string) string {
	if esPreV1.MatchString(version) {
		return "/_cluster/nodes/_local/stats"
	}
	return "/_nodes/_local/stats"
}

type esStatus struct {
	Status  int    `json:"status"`
	Name    string `json:"name"`
	Version struct {
		Number string `json:"number"`
	} `json:"version"`
}

type esStats struct {
	ClusterName string `json:"cluster_name"`
	Nodes       map[string]struct {
		Indices   map[string]map[string]interface{} `json:"indices"`
		Process   map[string]interface{}            `json:"process"`
		JVM       map[string]interface{}            `json:"jvm"`
		Network   map[string]interface{}            `json:"network"`
		Transport map[string]interface{}            `json:"transport"`
		HTTP      map[string]interface{}            `json:"http"`
	} `json:"nodes"`
}

type esClusterState struct {
	MasterNode string `json:"master_node"`
}
