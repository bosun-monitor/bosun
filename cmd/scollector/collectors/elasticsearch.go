package collectors

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_elasticsearch, Enable: enableURL(esURL)})
	collectors = append(collectors, &IntervalCollector{F: c_elasticsearch_indices, Interval: time.Minute * 2, Enable: enableURL(esURL)})
}

const esURL = "http://localhost:9200/"

var (
	esPreV1     = regexp.MustCompile(`^0\.`)
	esStatusMap = map[string]int{
		"green":  0,
		"yellow": 1,
		"red":    2,
	}
	esIndexFilters = make([]*regexp.Regexp, 0)
)

func AddElasticIndexFilter(s string) error {
	re, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	esIndexFilters = append(esIndexFilters, re)
	return nil
}

func c_elasticsearch() (opentsdb.MultiDataPoint, error) {
	var status esStatus
	if err := esReq("/", "", &status); err != nil {
		return nil, err
	}
	var stats esStats
	if err := esReq(esStatsURL(status.Version.Number), "", &stats); err != nil {
		return nil, err
	}
	var clusterState esClusterState
	if err := esReq("/_cluster/state", "?filter_routing_table=true&filter_metadata=true&filter_blocks=true", &clusterState); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	add := func(name string, val interface{}, ts opentsdb.TagSet) {
		tags := opentsdb.TagSet{"cluster": stats.ClusterName}
		for k, v := range ts {
			tags[k] = v
		}
		Add(&md, "elastic."+name, val, tags, metadata.Unknown, metadata.None, "")
	}
	for nodeid, nstats := range stats.Nodes {
		isMaster := nodeid == clusterState.MasterNode
		if isMaster {
			cstats := make(map[string]interface{})
			if err := esReq("/_cluster/health", "", &cstats); err != nil {
				return nil, err
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
	return md, nil
}

type ElasticIndexStats struct {
	All    ElasticIndex `json:"_all"`
	Shards struct {
		Failed     float64 `json:"failed"`
		Successful float64 `json:"successful"`
		Total      float64 `json:"total"`
	} `json:"_shards"`
	Indices map[string]ElasticIndex `json:"indices"`
}

type ElasticIndex struct {
	Primaries ElasticIndexDetails `json:"primaries"`
	Total     ElasticIndexDetails `json:"total"`
}

type ElasticIndexDetails struct {
	Completion struct {
		SizeInBytes float64 `json:"size_in_bytes"`
	} `json:"completion"`
	Docs struct {
		Count   float64 `json:"count"`
		Deleted float64 `json:"deleted"`
	} `json:"docs"`
	Fielddata struct {
		Evictions         float64 `json:"evictions"`
		MemorySizeInBytes float64 `json:"memory_size_in_bytes"`
	} `json:"fielddata"`
	FilterCache struct {
		Evictions         float64 `json:"evictions"`
		MemorySizeInBytes float64 `json:"memory_size_in_bytes"`
	} `json:"filter_cache"`
	Flush struct {
		Total             float64 `json:"total"`
		TotalTimeInMillis float64 `json:"total_time_in_millis"`
	} `json:"flush"`
	Get struct {
		Current             float64 `json:"current"`
		ExistsTimeInMillis  float64 `json:"exists_time_in_millis"`
		ExistsTotal         float64 `json:"exists_total"`
		MissingTimeInMillis float64 `json:"missing_time_in_millis"`
		MissingTotal        float64 `json:"missing_total"`
		TimeInMillis        float64 `json:"time_in_millis"`
		Total               float64 `json:"total"`
	} `json:"get"`
	IDCache struct {
		MemorySizeInBytes float64 `json:"memory_size_in_bytes"`
	} `json:"id_cache"`
	Indexing struct {
		DeleteCurrent      float64 `json:"delete_current"`
		DeleteTimeInMillis float64 `json:"delete_time_in_millis"`
		DeleteTotal        float64 `json:"delete_total"`
		IndexCurrent       float64 `json:"index_current"`
		IndexTimeInMillis  float64 `json:"index_time_in_millis"`
		IndexTotal         float64 `json:"index_total"`
	} `json:"indexing"`
	Merges struct {
		Current            float64 `json:"current"`
		CurrentDocs        float64 `json:"current_docs"`
		CurrentSizeInBytes float64 `json:"current_size_in_bytes"`
		Total              float64 `json:"total"`
		TotalDocs          float64 `json:"total_docs"`
		TotalSizeInBytes   float64 `json:"total_size_in_bytes"`
		TotalTimeInMillis  float64 `json:"total_time_in_millis"`
	} `json:"merges"`
	Percolate struct {
		Current           float64 `json:"current"`
		MemorySize        string  `json:"memory_size"`
		MemorySizeInBytes float64 `json:"memory_size_in_bytes"`
		Queries           float64 `json:"queries"`
		TimeInMillis      float64 `json:"time_in_millis"`
		Total             float64 `json:"total"`
	} `json:"percolate"`
	Refresh struct {
		Total             float64 `json:"total"`
		TotalTimeInMillis float64 `json:"total_time_in_millis"`
	} `json:"refresh"`
	Search struct {
		FetchCurrent      float64 `json:"fetch_current"`
		FetchTimeInMillis float64 `json:"fetch_time_in_millis"`
		FetchTotal        float64 `json:"fetch_total"`
		OpenContexts      float64 `json:"open_contexts"`
		QueryCurrent      float64 `json:"query_current"`
		QueryTimeInMillis float64 `json:"query_time_in_millis"`
		QueryTotal        float64 `json:"query_total"`
	} `json:"search"`
	Segments struct {
		Count         float64 `json:"count"`
		MemoryInBytes float64 `json:"memory_in_bytes"`
	} `json:"segments"`
	Store struct {
		SizeInBytes          float64 `json:"size_in_bytes"`
		ThrottleTimeInMillis float64 `json:"throttle_time_in_millis"`
	} `json:"store"`
	Suggest struct {
		Current      float64 `json:"current"`
		TimeInMillis float64 `json:"time_in_millis"`
		Total        float64 `json:"total"`
	} `json:"suggest"`
	Translog struct {
		Operations  float64 `json:"operations"`
		SizeInBytes float64 `json:"size_in_bytes"`
	} `json:"translog"`
	Warmer struct {
		Current           float64 `json:"current"`
		Total             float64 `json:"total"`
		TotalTimeInMillis float64 `json:"total_time_in_millis"`
	} `json:"warmer"`
}

const (
	descCompletionSizeInBytes        = "Size of the completion index (used for auto-complete functionallity)."
	descDocsCount                    = "The number of documents in the index."
	descDocsDeleted                  = "The number of deleted documents in the index."
	descFielddataEvictions           = "The number of cache evictions for field data."
	descFielddataMemorySizeInBytes   = "The amount of memory used for field data."
	descFilterCacheEvictions         = "The number of cache evictions for filter data."
	descFilterCacheMemorySizeInBytes = "The amount of memory used for filter data."
	descFlushTotal                   = "The number of flush operations. The flush process of an index basically frees memory from the index by flushing data to the index storage and clearing the internal transaction log."
	descFlushTotalTimeInMillis       = "The total amount of time spent on flush operations. The flush process of an index basically frees memory from the index by flushing data to the index storage and clearing the internal transaction log."
	descGetCurrent                   = "The current number of get operations. Gets get a typed JSON document from the index based on its id."
	descGetTimeInMillis              = "The total amount of time spent on get operations. Gets get a typed JSON document from the index based on its id."
	descGetTotal                     = "The total number of get operations. Gets get a typed JSON document from the index based on its id."
	descGetMissingTimeInMillis       = "The total amount of time spent trying to get documents that turned out to be missing."
	descGetMissingTotal              = "The total number of operations that tried to get a document that turned out to be missing."
	descGetExistsTimeInMillis        = "The total amount of time spent on get exists operations. Gets exists sees if a document exists."
	descGetExistsTotal               = "The total number of get exists operations. Gets exists sees if a document exists."
	descIDCacheMemorySizeInBytes     = "The size of the id cache."
	descIndexingDeleteCurrent        = "The current number of documents being deleted via indexing commands (such as a delete query)."
	descIndexingDeleteTimeInMillis   = "The time spent deleting documents."
	descIndexingDeleteTotal          = "The total number of documents deleted."
	descIndexingIndexCurrent         = "The current number of documents being indexed."
	descIndexingIndexTimeInMillis    = "The total amount of time spent indexing documents."
	descIndexingIndexTotal           = "The total number of documents indexed."
	descMergesCurrent                = "The current number of merge operations. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesCurrentDocs            = "The current number of documents that have an underlying merge operation going on. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesCurrentSizeInBytes     = "The current number of bytes being merged. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesTotal                  = "The total number of merges. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesTotalDocs              = "The total number of documents that have had an underlying merge operation. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesTotalSizeInBytes       = "The total number of bytes merged. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descMergesTotalTimeInMillis      = "The total amount of time spent on merge operations. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."
	descPercolateCurrent             = "The current number of percolate operations."
	descPercolateMemorySizeInBytes   = "The amount of memory used for the percolate index. Percolate is a reverse query to document operation."
	descPercolateQueries             = "The total number of percolate queries. Percolate is a reverse query to document operation."
	descPercolateTimeInMillis        = "The total amount of time spent on percolating. Percolate is a reverse query to document operation."
	descPercolateTotal               = "The total number of percolate operations. Percolate is a reverse query to document operation."
	descRefreshTotal                 = "The total number of refreshes. Refreshing makes all operations performed since the last search available."
	descRefreshTotalTimeInMillis     = "The total amount of time spent on refreshes. Refreshing makes all operations performed since the last search available."
	descSearchFetchCurrent           = "The current number of documents being fetched. Fetching is a phase of querying in a distributed search."
	descSearchFetchTimeInMillis      = "The total time spent fetching documents. Fetching is a phase of querying in a distributed search."
	descSearchFetchTotal             = "The total number of documents fetched. Fetching is a phase of querying in a distributed search."
	descSearchOpenContexts           = "The current number of open contexts. A search is left open when srolling (i.e. pagination)."
	descSearchQueryCurrent           = "The current number of queries."
	descSearchQueryTimeInMillis      = "The total amount of time spent querying."
	descSearchQueryTotal             = "The total number of queries."
	descSegmentsMemoryInBytes        = "The total amount of memory used for Lucene segments."
	descSegmentsCount                = "The number of segments that make up the index."
	descStoreSizeInBytes             = "The current size of the store."
	descStoreThrottleTimeInMillis    = "The amount of time that merges where throttled."
	descSuggestCurrent               = "The current number of suggest operations."
	descSuggestTimeInMillis          = "The total amount of time spent on suggest operations."
	descSuggestTotal                 = "The total number of suggest operations."
	descTranslogOperations           = "The total number of translog operations. The transaction logs (or write ahead logs) ensure atomicity of operations."
	descTranslogSizeInBytes          = "The current size of transaction log. The transaction log (or write ahead log) ensure atomicity of operations."
	descWarmerCurrent                = "The current number of warmer operations. Warming registers search requests in the background to speed up actual search requests."
	descWarmerTotal                  = "The total number of warmer operations. Warming registers search requests in the background to speed up actual search requests."
	descWarmerTotalTimeInMillis      = "The total time spent on warmer operations. Warming registers search requests in the background to speed up actual search requests."
)

type ElasticIndicesHealth struct {
	ActivePrimaryShards float64                       `json:"active_primary_shards"`
	ActiveShards        float64                       `json:"active_shards"`
	ClusterName         string                        `json:"cluster_name"`
	Indices             map[string]ElasticIndexHealth `json:"indices"`
	InitializingShards  float64                       `json:"initializing_shards"`
	NumberOfDataNodes   float64                       `json:"number_of_data_nodes"`
	NumberOfNodes       float64                       `json:"number_of_nodes"`
	RelocatingShards    float64                       `json:"relocating_shards"`
	Status              string                        `json:"status"`
	TimedOut            bool                          `json:"timed_out"`
	UnassignedShards    float64                       `json:"unassigned_shards"`
}

type ElasticIndexHealth struct {
	ActivePrimaryShards float64 `json:"active_primary_shards"`
	ActiveShards        float64 `json:"active_shards"`
	InitializingShards  float64 `json:"initializing_shards"`
	NumberOfReplicas    float64 `json:"number_of_replicas"`
	NumberOfShards      float64 `json:"number_of_shards"`
	RelocatingShards    float64 `json:"relocating_shards"`
	Status              string  `json:"status"`
	UnassignedShards    float64 `json:"unassigned_shards"`
}

const (
	descStatus              = "The current status of the index. Zero for green, one for yellow, two for red."
	descActivePrimaryShards = "The number of active primary shards. Each document is stored in a single primary shard and then when it is indexed it is copied the replicas of that shard."
	descActiveShards        = "The number of active shards."
	descInitializingShards  = "The number of initalizing shards."
	descNumberOfShards      = "The number of shards."
	descRelocatingShards    = "The number of shards relocating."
	descNumberOfReplicas    = "The number of replicas."
)

func esSkipIndex(index string) bool {
	for _, re := range esIndexFilters {
		if re.MatchString(index) {
			return true
		}
	}
	return false
}

func c_elasticsearch_indices() (opentsdb.MultiDataPoint, error) {
	var stats ElasticIndexStats
	var health ElasticIndicesHealth
	if err := esReq("/_cluster/health", "level=indices", &health); err != nil {
		return nil, err
	}
	cluster := health.ClusterName
	var md opentsdb.MultiDataPoint
	for k, v := range health.Indices {
		if esSkipIndex(k) {
			continue
		}
		ts := opentsdb.TagSet{"index_name": k, "cluster": cluster}
		if status, ok := esStatusMap[v.Status]; ok {
			Add(&md, "elastic.indices.status", status, ts, metadata.Gauge, metadata.StatusCode, descStatus)
		}
		Add(&md, "elastic.indices.shards.active_primary", v.ActivePrimaryShards, ts, metadata.Gauge, metadata.Shard, descActivePrimaryShards)
		Add(&md, "elastic.indices.shards.active", v.ActiveShards, ts, metadata.Gauge, metadata.Shard, descActiveShards)
		Add(&md, "elastic.indices.shards.initalizing", v.InitializingShards, ts, metadata.Gauge, metadata.Shard, descInitializingShards)
		Add(&md, "elastic.indices.shards.number", v.NumberOfShards, ts, metadata.Gauge, metadata.Shard, descNumberOfShards)
		Add(&md, "elastic.indices.shards.relocating", v.RelocatingShards, ts, metadata.Gauge, metadata.Shard, descRelocatingShards)
		Add(&md, "elastic.indices.replicas", v.NumberOfReplicas, ts, metadata.Gauge, metadata.Replica, descNumberOfReplicas)

	}
	if err := esReq("/_stats", "", &stats); err != nil {
		return nil, err
	}
	for k, v := range stats.Indices {
		if esSkipIndex(k) {
			continue
		}
		ts := opentsdb.TagSet{"index_name": k, "cluster": cluster}
		Add(&md, "elastic.indices.completion.size", v.Primaries.Completion.SizeInBytes, ts, metadata.Gauge, metadata.Bytes, descCompletionSizeInBytes)
		Add(&md, "elastic.indices.docs.count", v.Primaries.Docs.Count, ts, metadata.Gauge, metadata.Document, descDocsCount)
		Add(&md, "elastic.indices.docs.deleted", v.Primaries.Docs.Deleted, ts, metadata.Gauge, metadata.Document, descDocsDeleted)
		Add(&md, "elastic.indices.fielddata.evictions", v.Primaries.Fielddata.Evictions, ts, metadata.Counter, metadata.Eviction, descFielddataEvictions)
		Add(&md, "elastic.indices.fielddata.memory_size", v.Primaries.Fielddata.MemorySizeInBytes, ts, metadata.Gauge, metadata.Bytes, descFielddataMemorySizeInBytes)
		Add(&md, "elastic.indices.filter_cache.evictions", v.Primaries.FilterCache.Evictions, ts, metadata.Counter, metadata.Eviction, descFilterCacheEvictions)
		Add(&md, "elastic.indices.filter_cache.memory_size", v.Primaries.FilterCache.MemorySizeInBytes, ts, metadata.Counter, metadata.Bytes, descFilterCacheMemorySizeInBytes)
		Add(&md, "elastic.indices.flush.total", v.Primaries.Flush.Total, ts, metadata.Counter, metadata.Flush, descFlushTotal)
		Add(&md, "elastic.indices.flush.total_time", v.Primaries.Flush.TotalTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descFlushTotalTimeInMillis)
		Add(&md, "elastic.indices.get.current", v.Primaries.Get.Current, ts, metadata.Gauge, metadata.Get, descGetCurrent)
		Add(&md, "elastic.indices.get.exists_time", v.Primaries.Get.ExistsTimeInMillis, ts, metadata.Counter, metadata.GetExists, descGetExistsTimeInMillis)
		Add(&md, "elastic.indices.get.exists_total", v.Primaries.Get.ExistsTotal, ts, metadata.Counter, metadata.GetExists, descGetExistsTotal)
		Add(&md, "elastic.indices.get.missing_time", v.Primaries.Get.MissingTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descGetMissingTimeInMillis)
		Add(&md, "elastic.indices.get.missing_total", v.Primaries.Get.MissingTotal, ts, metadata.Counter, metadata.Operation, descGetMissingTotal)
		Add(&md, "elastic.indices.get.time", v.Primaries.Get.TimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descGetTimeInMillis)
		Add(&md, "elastic.indices.get.total", v.Primaries.Get.Total, ts, metadata.Counter, metadata.Get, descGetTotal)
		Add(&md, "elastic.indices.id_cache.memory_size", v.Primaries.IDCache.MemorySizeInBytes, ts, metadata.Gauge, metadata.Bytes, descIDCacheMemorySizeInBytes)
		Add(&md, "elastic.indices.indexing.delete_current", v.Primaries.Indexing.DeleteCurrent, ts, metadata.Gauge, metadata.Document, descIndexingDeleteCurrent)
		Add(&md, "elastic.indices.indexing.delete_time", v.Primaries.Indexing.DeleteTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descIndexingDeleteTimeInMillis)
		Add(&md, "elastic.indices.indexing.delete_total", v.Primaries.Indexing.DeleteTotal, ts, metadata.Counter, metadata.Document, descIndexingDeleteTotal)
		Add(&md, "elastic.indices.indexing.index_current", v.Primaries.Indexing.IndexCurrent, ts, metadata.Gauge, metadata.Document, descIndexingIndexCurrent)
		Add(&md, "elastic.indices.indexing.index_time", v.Primaries.Indexing.IndexTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descIndexingIndexTimeInMillis)
		Add(&md, "elastic.indices.indexing.index_total", v.Primaries.Indexing.IndexTotal, ts, metadata.Counter, metadata.Document, descIndexingIndexTotal)
		Add(&md, "elastic.indices.merges.current", v.Primaries.Merges.Current, ts, metadata.Gauge, metadata.Merge, descMergesCurrent)
		Add(&md, "elastic.indices.merges.current_docs", v.Primaries.Merges.CurrentDocs, ts, metadata.Gauge, metadata.Document, descMergesCurrentDocs)
		Add(&md, "elastic.indices.merges.current_size", v.Primaries.Merges.CurrentSizeInBytes, ts, metadata.Gauge, metadata.Document, descMergesCurrentSizeInBytes)
		Add(&md, "elastic.indices.merges.total", v.Primaries.Merges.Total, ts, metadata.Counter, metadata.Merge, descMergesTotal)
		Add(&md, "elastic.indices.merges.total_docs", v.Primaries.Merges.TotalDocs, ts, metadata.Counter, metadata.Document, descMergesTotalDocs)
		Add(&md, "elastic.indices.merges.total_size", v.Primaries.Merges.TotalSizeInBytes, ts, metadata.Counter, metadata.Bytes, descMergesTotalSizeInBytes)
		Add(&md, "elastic.indices.merges.total_time", v.Primaries.Merges.TotalTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descMergesTotalTimeInMillis)
		Add(&md, "elastic.indices.percolate.current", v.Primaries.Percolate.Current, ts, metadata.Gauge, "", descPercolateCurrent)
		Add(&md, "elastic.indices.percolate.memory_size", v.Primaries.Percolate.MemorySizeInBytes, ts, metadata.Gauge, metadata.Bytes, descPercolateMemorySizeInBytes)
		Add(&md, "elastic.indices.percolate.queries", v.Primaries.Percolate.Queries, ts, metadata.Counter, metadata.Query, descPercolateQueries)
		Add(&md, "elastic.indices.percolate.time", v.Primaries.Percolate.TimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descPercolateTimeInMillis)
		Add(&md, "elastic.indices.percolate.total", v.Primaries.Percolate.Total, ts, metadata.Gauge, metadata.Operation, descPercolateTotal)
		Add(&md, "elastic.indices.refresh.total", v.Primaries.Refresh.Total, ts, metadata.Counter, metadata.Refresh, descRefreshTotal)
		Add(&md, "elastic.indices.refresh.total_time", v.Primaries.Refresh.TotalTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descRefreshTotalTimeInMillis)
		Add(&md, "elastic.indices.search.fetch_current", v.Primaries.Search.FetchCurrent, ts, metadata.Gauge, metadata.Document, descSearchFetchCurrent)
		Add(&md, "elastic.indices.search.fetch_time", v.Primaries.Search.FetchTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descSearchFetchTimeInMillis)
		Add(&md, "elastic.indices.search.fetch_total", v.Primaries.Search.FetchTotal, ts, metadata.Counter, metadata.Document, descSearchFetchTotal)
		Add(&md, "elastic.indices.search.open_contexts", v.Primaries.Search.OpenContexts, ts, metadata.Gauge, metadata.Context, descSearchOpenContexts)
		Add(&md, "elastic.indices.search.query_current", v.Primaries.Search.QueryCurrent, ts, metadata.Gauge, metadata.Query, descSearchQueryCurrent)
		Add(&md, "elastic.indices.search.query_time", v.Primaries.Search.QueryTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descSearchQueryTimeInMillis)
		Add(&md, "elastic.indices.search.query_total", v.Primaries.Search.QueryTotal, ts, metadata.Counter, metadata.Query, descSearchQueryTotal)
		Add(&md, "elastic.indices.segments.count", v.Primaries.Segments.Count, ts, metadata.Counter, metadata.Segment, descSegmentsCount)
		Add(&md, "elastic.indices.segments.memory", v.Primaries.Segments.MemoryInBytes, ts, metadata.Gauge, metadata.Bytes, descSegmentsMemoryInBytes)
		Add(&md, "elastic.indices.store.size_in_bytes", v.Primaries.Store.SizeInBytes, ts, metadata.Gauge, metadata.Bytes, descStoreSizeInBytes)
		Add(&md, "elastic.indices.store.throttle_time", v.Primaries.Store.ThrottleTimeInMillis, ts, metadata.Gauge, metadata.MilliSecond, descStoreThrottleTimeInMillis)
		Add(&md, "elastic.indices.suggest.current", v.Primaries.Suggest.Current, ts, metadata.Gauge, metadata.Suggest, descSuggestCurrent)
		Add(&md, "elastic.indices.suggest.time", v.Primaries.Suggest.TimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descSuggestTimeInMillis)
		Add(&md, "elastic.indices.suggest.total", v.Primaries.Suggest.Total, ts, metadata.Counter, metadata.Suggest, descSuggestTotal)
		Add(&md, "elastic.indices.translog.operations", v.Primaries.Translog.Operations, ts, metadata.Counter, metadata.Operation, descTranslogOperations)
		Add(&md, "elastic.indices.translog.size_in_bytes", v.Primaries.Translog.SizeInBytes, ts, metadata.Gauge, metadata.Bytes, descTranslogSizeInBytes)
		Add(&md, "elastic.indices.warmer.current", v.Primaries.Warmer.Current, ts, metadata.Gauge, metadata.Operation, descWarmerCurrent)
		Add(&md, "elastic.indices.warmer.total", v.Primaries.Warmer.Total, ts, metadata.Counter, metadata.Operation, descWarmerTotal)
		Add(&md, "elastic.indices.warmer.total_time", v.Primaries.Warmer.TotalTimeInMillis, ts, metadata.Counter, metadata.MilliSecond, descWarmerTotalTimeInMillis)
	}
	return md, nil
}

func esReq(path, query string, v interface{}) error {
	u := &url.URL{
		Scheme:   "http",
		Host:     "localhost:9200",
		Path:     path,
		RawQuery: query,
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	j := json.NewDecoder(resp.Body)
	return j.Decode(v)
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

func divInterfaceFlt(a, b interface{}) (float64, error) {
	af, ok := a.(float64)
	if !ok {
		return 0, errors.New("elasticsearch: expected float64")
	}
	bf, ok := b.(float64)
	if !ok {
		return 0, errors.New("elasticsearch: expected float64")
	}
	r := af / bf
	if math.IsNaN(r) {
		return 0, errors.New("elasticsearch: got NaN")
	} else if math.IsInf(r, 0) {
		return 0, errors.New("elasticsearch: got Inf")
	}
	return r, nil
}
