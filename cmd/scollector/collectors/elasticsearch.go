package collectors

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, filter := range c.ElasticIndexFilters {
			err := AddElasticIndexFilter(filter, true)
			if err != nil {
				slog.Errorf("Error processing ElasticIndexFilter: %s", err)
			}
		}
		for _, filter := range c.ElasticIndexFiltersInc {
			err := AddElasticIndexFilter(filter, false)
			if err != nil {
				slog.Errorf("Error processing ElasticIndexFilterInc: %s", err)
			}
		}
		if c.Elastic == nil {
			// preserve the legacy defaults (localhost:9200)
			c.Elastic = append(c.Elastic, conf.Elastic{})
		}
		for _, instance := range c.Elastic {
			var indexInterval = time.Minute * 15
			var clusterInterval = DefaultFreq
			var err error
			// preserve defaults of localhost:9200
			if instance.Host == "" {
				instance.Host = "localhost"
			}
			if instance.Port == 0 {
				instance.Port = 9200
			}
			if instance.Scheme == "" {
				instance.Scheme = "http"
			}
			if instance.Name == "" {
				instance.Name = fmt.Sprintf("%v_%v", instance.Host, instance.Port)
			}
			if instance.Disable {
				slog.Infof("Elastic instance %v is disabled. Skipping.", instance.Name)
				continue
			}
			var creds string
			if instance.User != "" || instance.Password != "" {
				creds = fmt.Sprintf("%v:%v@", instance.User, instance.Password)
			} else {
				creds = ""
			}
			url := fmt.Sprintf("%v://%v%v:%v", instance.Scheme, creds, instance.Host, instance.Port)
			if instance.IndexInterval != "" {
				indexInterval, err = time.ParseDuration(instance.IndexInterval)
				if err != nil {
					panic(fmt.Errorf("Failed to parse IndexInterval: %v, err: %v", instance.IndexInterval, err))
				}
				slog.Infof("Using IndexInterval: %v for %v", indexInterval, instance.Name)
			} else {
				slog.Infof("Using default IndexInterval: %v for %v", indexInterval, instance.Name)
			}
			if instance.ClusterInterval != "" {
				clusterInterval, err = time.ParseDuration(instance.ClusterInterval)
				if err != nil {
					panic(fmt.Errorf("Failed to parse ClusterInterval: %v, err: %v", instance.ClusterInterval, err))
				}
				slog.Infof("Using ClusterInterval: %v for %v", clusterInterval, instance.Name)
			} else {
				slog.Infof("Using default ClusterInterval: %v for %v", clusterInterval, instance.Name)
			}
			// for legacy reasons, keep localhost:9200 named elasticsearch / elasticsearch-indices
			var name string
			if instance.Name == "localhost_9200" {
				name = "elasticsearch"
			} else {
				name = fmt.Sprintf("elasticsearch-%v", instance.Name)
			}
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_elasticsearch(false, instance)
				},
				name:     name,
				Interval: clusterInterval,
				Enable:   enableURL(url),
			})
			// keep legacy collector name if localhost_9200
			if instance.Name == "localhost_9200" {
				name = "elasticsearch-indices"
			} else {
				name = fmt.Sprintf("elasticsearch-indices-%v", instance.Name)
			}
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_elasticsearch(true, instance)
				},
				name:     name,
				Interval: indexInterval,
				Enable:   enableURL(url),
			})
		}
	})
}

var (
	elasticPreV1     = regexp.MustCompile(`^0\.`)
	elasticStatusMap = map[string]int{
		"green":  0,
		"yellow": 1,
		"red":    2,
	}
	elasticIndexFilters    = make([]*regexp.Regexp, 0)
	elasticIndexFiltersInc = make([]*regexp.Regexp, 0)
)

func AddElasticIndexFilter(s string, exclude bool) error {
	re, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	if exclude {
		slog.Infof("Added ES Index Filter: %v", s)
		elasticIndexFilters = append(elasticIndexFilters, re)
	} else {
		slog.Infof("Added ES Index Inclusion Filter: %v", s)
		elasticIndexFiltersInc = append(elasticIndexFiltersInc, re)
	}
	return nil
}

type structProcessor struct {
	elasticVersion string
	md             *opentsdb.MultiDataPoint
}

// structProcessor.add() takes in a metric name prefix, an arbitrary struct, and a tagset.
// The processor recurses through the struct and builds metrics. The field tags direct how
// the field should be processed, as well as the metadata for the resulting metric.
//
// The field tags used are described as follows:
//
// version: typically set to '1' or '2'.
//	This is compared against the elastic cluster version. If the version from the tag
//      does not match the version in production, the metric will not be sent for this field.
//
// exclude:
//      If this tag is set to 'true', a metric will not be sent for this field.
//
// rate: one of 'gauge', 'counter', 'rate'
//	This tag dictates the metadata.RateType we send.
//
// unit: 'bytes', 'pages', etc
//	This tag dictates the metadata.Unit we send.
//
// metric:
//      This is the metric name which will be sent. If not present, the 'json'
//      tag is sent as the metric name.
//
// Special handling:
//
// Metrics having the json tag suffix of 'in_milliseconds' are automagically
// divided by 1000 and sent as seconds. The suffix is stripped from the name.
//
// Metrics having the json tag suffix of 'in_bytes' are automatically sent as
// gauge bytes. The suffix is stripped from the metric name.
func (s *structProcessor) add(prefix string, st interface{}, ts opentsdb.TagSet) {
	t := reflect.TypeOf(st)
	valueOf := reflect.ValueOf(st)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := valueOf.Field(i).Interface()
		if field.Tag.Get("exclude") == "true" {
			continue
		}
		var (
			jsonTag    = field.Tag.Get("json")
			metricTag  = field.Tag.Get("metric")
			versionTag = field.Tag.Get("version")
			rateTag    = field.Tag.Get("rate")
			unitTag    = field.Tag.Get("unit")
		)
		metricName := jsonTag
		if metricTag != "" {
			metricName = metricTag
		}
		if metricName == "" {
			slog.Errorf("Unable to determine metric name for field %s. Skipping.", field.Name)
			continue
		}
		if versionTag == "" || strings.HasPrefix(s.elasticVersion, versionTag) {
			switch value := value.(type) {
			case int, float64: // Number types in our structs are only ints and float64s.
				// Turn all millisecond metrics into seconds
				if strings.HasSuffix(metricName, "_in_millis") {
					switch value.(type) {
					case int:
						value = float64(value.(int)) / 1000
					case float64:
						value = value.(float64) / 1000
					}
					unitTag = "seconds"
					metricName = strings.TrimSuffix(metricName, "_in_millis")
				}
				// Set rate and unit for all "_in_bytes" metrics, and strip the "_in_bytes"
				if strings.HasSuffix(metricName, "_in_bytes") {
					if rateTag == "" {
						rateTag = "gauge"
					}
					unitTag = "bytes"
					metricName = strings.TrimSuffix(metricName, "_in_bytes")
				}
				Add(s.md, prefix+"."+metricName, value, ts, metadata.RateType(rateTag), metadata.Unit(unitTag), field.Tag.Get("desc"))
			case string:
				// The json data has a lot of strings, and we don't care about em.
			default:
				// If we hit another struct, recurse
				if reflect.ValueOf(value).Kind() == reflect.Struct {
					s.add(prefix+"."+metricName, value, ts)
				} else {
					slog.Errorf("Field %s for metric %s is non-numeric type. Cannot record as a metric.\n", field.Name, prefix+"."+metricName)
				}
			}
		}
	}
}

func c_elasticsearch(collectIndices bool, instance conf.Elastic) (opentsdb.MultiDataPoint, error) {
	slog.Infof("Updating ES stats for %v", instance)
	var status ElasticStatus
	if err := esReq(instance, "/", "", &status); err != nil {
		return nil, err
	}
	var clusterStats ElasticClusterStats
	if err := esReq(instance, esStatsURL(status.Version.Number), "", &clusterStats); err != nil {
		return nil, err
	}
	var clusterState ElasticClusterState
	if err := esReq(instance, "/_cluster/state/master_node", "", &clusterState); err != nil {
		return nil, err
	}
	var clusterHealth ElasticHealth
	if err := esReq(instance, "/_cluster/health", "level=indices", &clusterHealth); err != nil {
		return nil, err
	}
	var indexStats ElasticIndexStats
	if err := esReq(instance, "/_stats", "", &indexStats); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	s := structProcessor{elasticVersion: status.Version.Number, md: &md}
	ts := opentsdb.TagSet{"cluster": clusterStats.ClusterName}
	isMaster := false
	// As we're pulling _local stats here, this will only process 1 node.
	for nodeID, nodeStats := range clusterStats.Nodes {
		isMaster = nodeID == clusterState.MasterNode
		if isMaster {
			s.add("elastic.health.cluster", clusterHealth, ts)
			if statusCode, ok := elasticStatusMap[clusterHealth.Status]; ok {
				Add(&md, "elastic.health.cluster.status", statusCode, ts, metadata.Gauge, metadata.StatusCode, "The current status of the cluster. Zero for green, one for yellow, two for red.")
			}
			indexStatusCount := map[string]int{
				"green":  0,
				"yellow": 0,
				"red":    0,
			}
			for _, index := range clusterHealth.Indices {
				indexStatusCount[index.Status] += 1
			}
			for status, count := range indexStatusCount {
				Add(&md, "elastic.health.cluster.index_status_count", count, opentsdb.TagSet{"status": status}.Merge(ts), metadata.Gauge, metadata.Unit("indices"), "Index counts by status.")
			}
		}
		s.add("elastic", nodeStats, ts)
		// These are index stats in aggregate for this node.
		s.add("elastic.indices.local", nodeStats.Indices, ts)
		s.add("elastic.jvm.gc", nodeStats.JVM.GC.Collectors.Old, opentsdb.TagSet{"gc": "old"}.Merge(ts))
		s.add("elastic.jvm.gc", nodeStats.JVM.GC.Collectors.Young, opentsdb.TagSet{"gc": "young"}.Merge(ts))
	}
	if collectIndices && isMaster {
		for k, index := range indexStats.Indices {
			if esSkipIndex(k) {
				slog.Infof("Skipping index: %v", k)
				continue
			}
			slog.Infof("Pulling index stats: %v", k)
			ts := opentsdb.TagSet{"index_name": k, "cluster": clusterStats.ClusterName}
			if indexHealth, ok := clusterHealth.Indices[k]; ok {
				s.add("elastic.health.indices", indexHealth, ts)
				if status, ok := elasticStatusMap[indexHealth.Status]; ok {
					Add(&md, "elastic.health.indices.status", status, ts, metadata.Gauge, metadata.StatusCode, "The current status of the index. Zero for green, one for yellow, two for red.")
				}
			}
			s.add("elastic.indices.cluster", index.Primaries, ts)
		}
	}
	return md, nil
}

func esSkipIndex(index string) bool {
	for _, re := range elasticIndexFilters {
		if re.MatchString(index) {
			return true
		}
	}
	for _, re := range elasticIndexFiltersInc {
		if re.MatchString(index) {
			return false
		}
	}
	return len(elasticIndexFiltersInc) > 0
}

func esReq(instance conf.Elastic, path, query string, v interface{}) error {
	up := url.UserPassword(instance.User, instance.Password)
	u := &url.URL{
		Scheme:   instance.Scheme,
		User:     up,
		Host:     fmt.Sprintf("%v:%v", instance.Host, instance.Port),
		Path:     path,
		RawQuery: query,
	}
	resp, err := http.Get(u.String())
	if err != nil {
		slog.Errorf("Error querying Elasticsearch: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(ioutil.Discard, resp.Body, 512)
		return nil
	}
	j := json.NewDecoder(resp.Body)
	return j.Decode(v)
}

func esStatsURL(version string) string {
	if elasticPreV1.MatchString(version) {
		return "/_cluster/nodes/_local/stats"
	}
	return "/_nodes/_local/stats"
}

type ElasticHealth struct {
	ActivePrimaryShards         int                           `json:"active_primary_shards" desc:"The number of active primary shards. Each document is stored in a single primary shard and then when it is indexed it is copied the replicas of that shard."`
	ActiveShards                int                           `json:"active_shards" desc:"The number of active shards."`
	ActiveShardsPercentAsNumber float64                       `json:"active_shards_percent_as_number" version:"2"` // 2.0 only
	ClusterName                 string                        `json:"cluster_name"`
	DelayedUnassignedShards     int                           `json:"delayed_unassigned_shards" version:"2"` // 2.0 only
	Indices                     map[string]ElasticIndexHealth `json:"indices" exclude:"true"`
	InitializingShards          int                           `json:"initializing_shards" desc:"The number of initalizing shards."`
	NumberOfDataNodes           int                           `json:"number_of_data_nodes"`
	NumberOfInFlightFetch       int                           `json:"number_of_in_flight_fetch" version:"2"` // 2.0 only
	NumberOfNodes               int                           `json:"number_of_nodes"`
	NumberOfPendingTasks        int                           `json:"number_of_pending_tasks"`
	RelocatingShards            int                           `json:"relocating_shards" desc:"The number of shards relocating."`
	Status                      string                        `json:"status" desc:"The current status of the cluster. 0: green, 1: yellow, 2: red."`
	TaskMaxWaitingInQueueMillis int                           `json:"task_max_waiting_in_queue_millis" version:"2"` // 2.0 only
	TimedOut                    bool                          `json:"timed_out" exclude:"true"`
	UnassignedShards            int                           `json:"unassigned_shards" version:"2"` // 2.0 only
}

type ElasticIndexHealth struct {
	ActivePrimaryShards int    `json:"active_primary_shards" desc:"The number of active primary shards. Each document is stored in a single primary shard and then when it is indexed it is copied the replicas of that shard."`
	ActiveShards        int    `json:"active_shards" desc:"The number of active shards."`
	InitializingShards  int    `json:"initializing_shards" desc:"The number of initalizing shards."`
	NumberOfReplicas    int    `json:"number_of_replicas" desc:"The number of replicas."`
	NumberOfShards      int    `json:"number_of_shards" desc:"The number of shards."`
	RelocatingShards    int    `json:"relocating_shards" desc:"The number of shards relocating."`
	Status              string `json:"status" desc:"The current status of the index. 0: green, 1: yellow, 2: red."`
	UnassignedShards    int    `json:"unassigned_shards"`
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
		SizeInBytes int `json:"size_in_bytes" desc:"Size of the completion index (used for auto-complete functionallity)."`
	} `json:"completion"`
	Docs struct {
		Count   int `json:"count" rate:"gauge" rate:"gauge" unit:"documents" desc:"The number of documents in the index."`
		Deleted int `json:"deleted" rate:"gauge" unit:"documents" desc:"The number of deleted documents in the index."`
	} `json:"docs"`
	Fielddata struct {
		Evictions         int `json:"evictions" rate:"counter" unit:"evictions" desc:"The number of cache evictions for field data."`
		MemorySizeInBytes int `json:"memory_size_in_bytes" desc:"The amount of memory used for field data."`
	} `json:"fielddata"`
	FilterCache struct { // 1.0 only
		Evictions         int `json:"evictions" version:"1" rate:"counter" unit:"evictions" desc:"The number of cache evictions for filter data."` // 1.0 only
		MemorySizeInBytes int `json:"memory_size_in_bytes" version:"1" desc:"The amount of memory used for filter data."`                          // 1.0 only
	} `json:"filter_cache"`
	Flush struct {
		Total             int `json:"total" rate:"counter" unit:"flushes" desc:"The number of flush operations. The flush process of an index basically frees memory from the index by flushing data to the index storage and clearing the internal transaction log."`
		TotalTimeInMillis int `json:"total_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on flush operations. The flush process of an index basically frees memory from the index by flushing data to the index storage and clearing the internal transaction log."`
	} `json:"flush"`
	Get struct {
		Current             int `json:"current" rate:"gauge" unit:"gets" desc:"The current number of get operations. Gets get a typed JSON document from the index based on its id."`
		ExistsTimeInMillis  int `json:"exists_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on get exists operations. Gets exists sees if a document exists."`
		ExistsTotal         int `json:"exists_total" rate:"counter" unit:"get exists" desc:"The total number of get exists operations. Gets exists sees if a document exists."`
		MissingTimeInMillis int `json:"missing_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent trying to get documents that turned out to be missing."`
		MissingTotal        int `json:"missing_total" rate:"counter" unit:"operations" desc:"The total number of operations that tried to get a document that turned out to be missing."`
		TimeInMillis        int `json:"time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on get operations. Gets get a typed JSON document from the index based on its id."`
		Total               int `json:"total" rate:"counter" unit:"operations" desc:"The total number of get operations. Gets get a typed JSON document from the index based on its id."`
	} `json:"get"`
	IDCache struct { // 1.0 only
		MemorySizeInBytes int `json:"memory_size_in_bytes" version:"1" desc:"The size of the id cache."` // 1.0 only
	} `json:"id_cache"`
	Indexing struct {
		DeleteCurrent        int  `json:"delete_current" rate:"gauge" unit:"documents" desc:"The current number of documents being deleted via indexing commands (such as a delete query)."`
		DeleteTimeInMillis   int  `json:"delete_time_in_millis" rate:"counter" unit:"seconds" desc:"The time spent deleting documents."`
		DeleteTotal          int  `json:"delete_total" rate:"counter" unit:"documents" desc:"The total number of documents deleted."`
		IndexCurrent         int  `json:"index_current" rate:"gauge" unit:"documents" desc:"The current number of documents being indexed."`
		IndexTimeInMillis    int  `json:"index_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent indexing documents."`
		IndexTotal           int  `json:"index_total" rate:"counter" unit:"documents" desc:"The total number of documents indexed."`
		IsThrottled          bool `json:"is_throttled" exclude:"true"`
		NoopUpdateTotal      int  `json:"noop_update_total"`
		ThrottleTimeInMillis int  `json:"throttle_time_in_millis"`
	} `json:"indexing"`
	Merges struct {
		Current                    int `json:"current" rate:"gauge" unit:"merges" desc:"The current number of merge operations. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		CurrentDocs                int `json:"current_docs" rate:"gauge" unit:"documents" desc:"The current number of documents that have an underlying merge operation going on. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		CurrentSizeInBytes         int `json:"current_size_in_bytes" desc:"The current number of bytes being merged. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		Total                      int `json:"total" rate:"counter" unit:"merges" desc:"The total number of merges. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		TotalAutoThrottleInBytes   int `json:"total_auto_throttle_in_bytes" version:"2"` // 2.0 only
		TotalDocs                  int `json:"total_docs" rate:"counter" unit:"documents" desc:"The total number of documents that have had an underlying merge operation. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		TotalSizeInBytes           int `json:"total_size_in_bytes" desc:"The total number of bytes merged. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
		TotalStoppedTimeInMillis   int `json:"total_stopped_time_in_millis" version:"2"`   // 2.0 only
		TotalThrottledTimeInMillis int `json:"total_throttled_time_in_millis" version:"2"` // 2.0 only
		TotalTimeInMillis          int `json:"total_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on merge operations. In elastic Lucene segments are merged behind the scenes. It is possible these can impact search performance."`
	} `json:"merges"`
	Percolate struct {
		Current           int    `json:"current" rate:"gauge" unit:"operations" desc:"The current number of percolate operations."`
		MemorySize        string `json:"memory_size"`
		MemorySizeInBytes int    `json:"memory_size_in_bytes" desc:"The amount of memory used for the percolate index. Percolate is a reverse query to document operation."`
		Queries           int    `json:"queries" rate:"counter" unit:"queries" desc:"The total number of percolate queries. Percolate is a reverse query to document operation."`
		TimeInMillis      int    `json:"time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on percolating. Percolate is a reverse query to document operation."`
		Total             int    `json:"total" rate:"gauge" unit:"operations" desc:"The total number of percolate operations. Percolate is a reverse query to document operation."`
	} `json:"percolate"`
	QueryCache struct {
		CacheCount        int `json:"cache_count" version:"2"` // 2.0 only
		CacheSize         int `json:"cache_size" version:"2"`  // 2.0 only
		Evictions         int `json:"evictions"`
		HitCount          int `json:"hit_count"`
		MemorySizeInBytes int `json:"memory_size_in_bytes"`
		MissCount         int `json:"miss_count"`
		TotalCount        int `json:"total_count" version:"2"` // 2.0 only
	} `json:"query_cache"`
	Recovery struct {
		CurrentAsSource      int `json:"current_as_source"`
		CurrentAsTarget      int `json:"current_as_target"`
		ThrottleTimeInMillis int `json:"throttle_time_in_millis"`
	} `json:"recovery"`
	Refresh struct {
		Total             int `json:"total" rate:"counter" unit:"refresh" desc:"The total number of refreshes. Refreshing makes all operations performed since the last search available."`
		TotalTimeInMillis int `json:"total_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent on refreshes. Refreshing makes all operations performed since the last search available."`
	} `json:"refresh"`
	RequestCache struct { // 2.0 only
		Evictions         int `json:"evictions" version:"2"`            // 2.0 only
		HitCount          int `json:"hit_count" version:"2"`            // 2.0 only
		MemorySizeInBytes int `json:"memory_size_in_bytes" version:"2"` // 2.0 only
		MissCount         int `json:"miss_count" version:"2"`           // 2.0 only
	} `json:"request_cache"`
	Search struct {
		FetchCurrent       int `json:"fetch_current" rate:"gauge" unit:"documents" desc:"The current number of documents being fetched. Fetching is a phase of querying in a distributed search."`
		FetchTimeInMillis  int `json:"fetch_time_in_millis" rate:"counter" unit:"seconds" desc:"The total time spent fetching documents. Fetching is a phase of querying in a distributed search."`
		FetchTotal         int `json:"fetch_total" rate:"counter" unit:"documents" desc:"The total number of documents fetched. Fetching is a phase of querying in a distributed search."`
		OpenContexts       int `json:"open_contexts" rate:"gauge" unit:"contexts" desc:"The current number of open contexts. A search is left open when srolling (i.e. pagination)."`
		QueryCurrent       int `json:"query_current" rate:"gauge" unit:"queries" desc:"The current number of queries."`
		QueryTimeInMillis  int `json:"query_time_in_millis" rate:"counter" unit:"seconds" desc:"The total amount of time spent querying."`
		QueryTotal         int `json:"query_total" rate:"counter" unit:"queries" desc:"The total number of queries."`
		ScrollCurrent      int `json:"scroll_current" version:"2"`        // 2.0 only
		ScrollTimeInMillis int `json:"scroll_time_in_millis" version:"2"` // 2.0 only
		ScrollTotal        int `json:"scroll_total" version:"2"`          // 2.0 only
	} `json:"search"`
	Segments struct {
		Count                       int `json:"count" rate:"counter" unit:"segments" desc:"The number of segments that make up the index."`
		DocValuesMemoryInBytes      int `json:"doc_values_memory_in_bytes" version:"2"` // 2.0 only
		FixedBitSetMemoryInBytes    int `json:"fixed_bit_set_memory_in_bytes"`
		IndexWriterMaxMemoryInBytes int `json:"index_writer_max_memory_in_bytes"`
		IndexWriterMemoryInBytes    int `json:"index_writer_memory_in_bytes"`
		MemoryInBytes               int `json:"memory_in_bytes" desc:"The total amount of memory used for Lucene segments."`
		NormsMemoryInBytes          int `json:"norms_memory_in_bytes" version:"2"`         // 2.0 only
		StoredFieldsMemoryInBytes   int `json:"stored_fields_memory_in_bytes" version:"2"` // 2.0 only
		TermVectorsMemoryInBytes    int `json:"term_vectors_memory_in_bytes" version:"2"`  // 2.0 only
		TermsMemoryInBytes          int `json:"terms_memory_in_bytes" version:"2"`         // 2.0 only
		VersionMapMemoryInBytes     int `json:"version_map_memory_in_bytes"`
	} `json:"segments"`
	Store struct {
		SizeInBytes          int `json:"size_in_bytes" unit:"bytes" desc:"The current size of the store."`
		ThrottleTimeInMillis int `json:"throttle_time_in_millis" rate:"gauge" unit:"seconds" desc:"The amount of time that merges where throttled."`
	} `json:"store"`
	Suggest struct {
		Current      int `json:"current" rate:"gauge" unit:"suggests" desc:"The current number of suggest operations."`
		TimeInMillis int `json:"time_in_millis" rate:"gauge" unit:"seconds" desc:"The total amount of time spent on suggest operations."`
		Total        int `json:"total" rate:"gauge" unit:"suggests" desc:"The total number of suggest operations."`
	} `json:"suggest"`
	Translog struct {
		Operations  int `json:"operations" rate:"gauge" unit:"operations" desc:"The total number of translog operations. The transaction logs (or write ahead logs) ensure atomicity of operations."`
		SizeInBytes int `json:"size_in_bytes" desc:"The current size of transaction log. The transaction log (or write ahead log) ensure atomicity of operations."`
	} `json:"translog"`
	Warmer struct {
		Current           int `json:"current" rate:"gauge" unit:"operations" desc:"The current number of warmer operations. Warming registers search requests in the background to speed up actual search requests."`
		Total             int `json:"total" rate:"gauge" unit:"operations" desc:"The total number of warmer operations. Warming registers search requests in the background to speed up actual search requests."`
		TotalTimeInMillis int `json:"total_time_in_millis" rate:"gauge" unit:"seconds" desc:"The total time spent on warmer operations. Warming registers search requests in the background to speed up actual search requests."`
	} `json:"warmer"`
}

type ElasticStatus struct {
	Status  int    `json:"status"`
	Name    string `json:"name"`
	Version struct {
		Number string `json:"number"`
	} `json:"version"`
}

type ElasticClusterStats struct {
	ClusterName string `json:"cluster_name"`
	Nodes       map[string]struct {
		Attributes struct {
			Master string `json:"master"`
		} `json:"attributes"`
		Breakers struct {
			Fielddata ElasticBreakersStat `json:"fielddata"`
			Parent    ElasticBreakersStat `json:"parent"`
			Request   ElasticBreakersStat `json:"request"`
		} `json:"breakers" exclude:"true"`
		FS struct {
			Data []struct {
				AvailableInBytes     int    `json:"available_in_bytes"`
				Dev                  string `json:"dev" version:"1"`                      // 1.0 only
				DiskIoOp             int    `json:"disk_io_op" version:"1"`               // 1.0 only
				DiskIoSizeInBytes    int    `json:"disk_io_size_in_bytes" version:"1"`    // 1.0 only
				DiskQueue            string `json:"disk_queue" version:"1"`               // 1.0 only
				DiskReadSizeInBytes  int    `json:"disk_read_size_in_bytes" version:"1"`  // 1.0 only
				DiskReads            int    `json:"disk_reads" version:"1"`               // 1.0 only
				DiskServiceTime      string `json:"disk_service_time" version:"1"`        // 1.0 only
				DiskWriteSizeInBytes int    `json:"disk_write_size_in_bytes" version:"1"` // 1.0 only
				DiskWrites           int    `json:"disk_writes" version:"1"`              // 1.0 only
				FreeInBytes          int    `json:"free_in_bytes"`
				Mount                string `json:"mount"`
				Path                 string `json:"path"`
				TotalInBytes         int    `json:"total_in_bytes"`
				Type                 string `json:"type" version:"2"` // 2.0 only
			} `json:"data"`
			Timestamp int `json:"timestamp"`
			Total     struct {
				AvailableInBytes     int    `json:"available_in_bytes"`
				DiskIoOp             int    `json:"disk_io_op" version:"1"`               // 1.0 only
				DiskIoSizeInBytes    int    `json:"disk_io_size_in_bytes" version:"1"`    // 1.0 only
				DiskQueue            string `json:"disk_queue" version:"1"`               // 1.0 only
				DiskReadSizeInBytes  int    `json:"disk_read_size_in_bytes" version:"1"`  // 1.0 only
				DiskReads            int    `json:"disk_reads" version:"1"`               // 1.0 only
				DiskServiceTime      string `json:"disk_service_time" version:"1"`        // 1.0 only
				DiskWriteSizeInBytes int    `json:"disk_write_size_in_bytes" version:"1"` // 1.0 only
				DiskWrites           int    `json:"disk_writes" version:"1"`              // 1.0 only
				FreeInBytes          int    `json:"free_in_bytes"`
				TotalInBytes         int    `json:"total_in_bytes"`
			} `json:"total"`
		} `json:"fs" exclude:"true"`
		Host string `json:"host"`
		HTTP struct {
			CurrentOpen int `json:"current_open"`
			TotalOpened int `json:"total_opened"`
		} `json:"http"`
		Indices ElasticIndexDetails `json:"indices" exclude:"true"` // Stored under elastic.indices.local namespace.
		//IP      []string            `json:"ip" exclude:"true"`	// Incompatible format between 5.x and previous, and not used in collector
		JVM struct {
			BufferPools struct {
				Direct struct {
					Count                int `json:"count"`
					TotalCapacityInBytes int `json:"total_capacity_in_bytes"`
					UsedInBytes          int `json:"used_in_bytes"`
				} `json:"direct"`
				Mapped struct {
					Count                int `json:"count"`
					TotalCapacityInBytes int `json:"total_capacity_in_bytes"`
					UsedInBytes          int `json:"used_in_bytes"`
				} `json:"mapped"`
			} `json:"buffer_pools"`
			Classes struct { // 2.0 only
				CurrentLoadedCount int `json:"current_loaded_count" version:"2"` // 2.0 only
				TotalLoadedCount   int `json:"total_loaded_count" version:"2"`   // 2.0 only
				TotalUnloadedCount int `json:"total_unloaded_count" version:"2"` // 2.0 only
			} `json:"classes"`
			GC struct {
				Collectors struct {
					Old struct {
						CollectionCount        int `json:"collection_count"`
						CollectionTimeInMillis int `json:"collection_time_in_millis"`
					} `json:"old"`
					Young struct {
						CollectionCount        int `json:"collection_count"`
						CollectionTimeInMillis int `json:"collection_time_in_millis"`
					} `json:"young"`
				} `json:"collectors"`
			} `json:"gc" exclude:"true"` // This is recorded manually so we can tag the GC collector type.
			Mem struct {
				HeapCommittedInBytes    int `json:"heap_committed_in_bytes" metric:"heap_committed"`
				HeapMaxInBytes          int `json:"heap_max_in_bytes"`
				HeapUsedInBytes         int `json:"heap_used_in_bytes" metric:"heap_used"`
				HeapUsedPercent         int `json:"heap_used_percent"`
				NonHeapCommittedInBytes int `json:"non_heap_committed_in_bytes"`
				NonHeapUsedInBytes      int `json:"non_heap_used_in_bytes"`
				Pools                   struct {
					Old struct {
						MaxInBytes      int `json:"max_in_bytes"`
						PeakMaxInBytes  int `json:"peak_max_in_bytes"`
						PeakUsedInBytes int `json:"peak_used_in_bytes"`
						UsedInBytes     int `json:"used_in_bytes"`
					} `json:"old"`
					Survivor struct {
						MaxInBytes      int `json:"max_in_bytes"`
						PeakMaxInBytes  int `json:"peak_max_in_bytes"`
						PeakUsedInBytes int `json:"peak_used_in_bytes"`
						UsedInBytes     int `json:"used_in_bytes"`
					} `json:"survivor"`
					Young struct {
						MaxInBytes      int `json:"max_in_bytes"`
						PeakMaxInBytes  int `json:"peak_max_in_bytes"`
						PeakUsedInBytes int `json:"peak_used_in_bytes"`
						UsedInBytes     int `json:"used_in_bytes"`
					} `json:"young"`
				} `json:"pools" exclude:"true"`
			} `json:"mem"`
			Threads struct {
				Count     int `json:"count"`
				PeakCount int `json:"peak_count"`
			} `json:"threads"`
			Timestamp      int `json:"timestamp"`
			UptimeInMillis int `json:"uptime_in_millis"`
		} `json:"jvm"`
		Name    string   `json:"name"`
		Network struct { // 1.0 only
			TCP struct { // 1.0 only
				ActiveOpens  int `json:"active_opens" version:"1"`  // 1.0 only
				AttemptFails int `json:"attempt_fails" version:"1"` // 1.0 only
				CurrEstab    int `json:"curr_estab" version:"1"`    // 1.0 only
				EstabResets  int `json:"estab_resets" version:"1"`  // 1.0 only
				InErrs       int `json:"in_errs" version:"1"`       // 1.0 only
				InSegs       int `json:"in_segs" version:"1"`       // 1.0 only
				OutRsts      int `json:"out_rsts" version:"1"`      // 1.0 only
				OutSegs      int `json:"out_segs" version:"1"`      // 1.0 only
				PassiveOpens int `json:"passive_opens" version:"1"` // 1.0 only
				RetransSegs  int `json:"retrans_segs" version:"1"`  // 1.0 only
			} `json:"tcp"`
		} `json:"network"`
		OS struct {
			CPU struct { // 1.0 only
				Idle   int `json:"idle" version:"1"`   // 1.0 only
				Stolen int `json:"stolen" version:"1"` // 1.0 only
				Sys    int `json:"sys" version:"1"`    // 1.0 only
				Usage  int `json:"usage" version:"1"`  // 1.0 only
				User   int `json:"user" version:"1"`   // 1.0 only
			} `json:"cpu"`
			//			LoadAverage []float64 `json:"load_average"` // 1.0 only
			//			LoadAverage float64 `json:"load_average"` // 2.0 only
			Mem struct {
				ActualFreeInBytes int `json:"actual_free_in_bytes" version:"1"` // 1.0 only
				ActualUsedInBytes int `json:"actual_used_in_bytes" version:"1"` // 1.0 only
				FreeInBytes       int `json:"free_in_bytes"`
				FreePercent       int `json:"free_percent"`
				TotalInBytes      int `json:"total_in_bytes" version:"2"` // 2.0 only
				UsedInBytes       int `json:"used_in_bytes"`
				UsedPercent       int `json:"used_percent"`
			} `json:"mem"`
			Swap struct {
				FreeInBytes  int `json:"free_in_bytes"`
				TotalInBytes int `json:"total_in_bytes" version:"2"` // 2.0 only
				UsedInBytes  int `json:"used_in_bytes"`
			} `json:"swap"`
			Timestamp      int `json:"timestamp"`
			UptimeInMillis int `json:"uptime_in_millis"`
		} `json:"os" exclude:"true"` // These are OS-wide stats, and are already gathered by other collectors.
		Process struct {
			CPU struct {
				Percent       int `json:"percent" exclude:"true"`
				SysInMillis   int `json:"sys_in_millis" version:"1"` // 1.0 only
				TotalInMillis int `json:"total_in_millis"`
				UserInMillis  int `json:"user_in_millis" version:"1"` // 1.0 only
			} `json:"cpu"`
			MaxFileDescriptors int `json:"max_file_descriptors" version:"2"` // 2.0 only
			Mem                struct {
				ResidentInBytes     int `json:"resident_in_bytes" metric:"resident" version:"1"` // 1.0 only
				ShareInBytes        int `json:"share_in_bytes" metric:"shared" version:"1"`      // 1.0 only
				TotalVirtualInBytes int `json:"total_virtual_in_bytes" metric:"total_virtual"`
			} `json:"mem"`
			OpenFileDescriptors int `json:"open_file_descriptors"`
			Timestamp           int `json:"timestamp" exclude:"true"`
		} `json:"process"`
		Script struct { // 2.0 only
			CacheEvictions int `json:"cache_evictions" version:"2"` // 2.0 only
			Compilations   int `json:"compilations" version:"2"`    // 2.0 only
		} `json:"script"`
		ThreadPool struct {
			Bulk              ElasticThreadPoolStat `json:"bulk"`
			FetchShardStarted ElasticThreadPoolStat `json:"fetch_shard_started" version:"2"` // 2.0 only
			FetchShardStore   ElasticThreadPoolStat `json:"fetch_shard_store" version:"2"`   // 2.0 only
			Flush             ElasticThreadPoolStat `json:"flush"`
			Generic           ElasticThreadPoolStat `json:"generic"`
			Get               ElasticThreadPoolStat `json:"get"`
			Index             ElasticThreadPoolStat `json:"index"`
			Listener          ElasticThreadPoolStat `json:"listener"`
			Management        ElasticThreadPoolStat `json:"management"`
			Merge             ElasticThreadPoolStat `json:"merge" version:"1"` // 1.0 only
			Optimize          ElasticThreadPoolStat `json:"optimize"`
			Percolate         ElasticThreadPoolStat `json:"percolate"`
			Refresh           ElasticThreadPoolStat `json:"refresh"`
			Search            ElasticThreadPoolStat `json:"search"`
			Snapshot          ElasticThreadPoolStat `json:"snapshot"`
			Suggest           ElasticThreadPoolStat `json:"suggest"`
			Warmer            ElasticThreadPoolStat `json:"warmer"`
		} `json:"thread_pool" exclude:"true"`
		Timestamp int `json:"timestamp"`
		Transport struct {
			RxCount       int `json:"rx_count"`
			RxSizeInBytes int `json:"rx_size_in_bytes"`
			ServerOpen    int `json:"server_open"`
			TxCount       int `json:"tx_count"`
			TxSizeInBytes int `json:"tx_size_in_bytes"`
		} `json:"transport"`
		TransportAddress string `json:"transport_address"`
	} `json:"nodes"`
}

type ElasticThreadPoolStat struct {
	Active    int `json:"active"`
	Completed int `json:"completed"`
	Largest   int `json:"largest"`
	Queue     int `json:"queue"`
	Rejected  int `json:"rejected"`
	Threads   int `json:"threads"`
}

type ElasticBreakersStat struct {
	EstimatedSize        string  `json:"estimated_size"`
	EstimatedSizeInBytes int     `json:"estimated_size_in_bytes"`
	LimitSize            string  `json:"limit_size"`
	LimitSizeInBytes     int     `json:"limit_size_in_bytes"`
	Overhead             float64 `json:"overhead"`
	Tripped              int     `json:"tripped"`
}

type ElasticClusterState struct {
	MasterNode string `json:"master_node"`
}
