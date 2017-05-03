package collectors

/**
	***We need a file : cache-status.php at the specified URL in the constants (normally at the web server root) :
	****************************************************************
	<?php
	
	// serve up some data about PHP's opcache and APCu subsystems as a JSON document
	
	// only permit access from localhost (to put in comments if testing on localhost)
	if (filter_input(INPUT_SERVER, 'REMOTE_ADDR') != '127.0.0.1') {
	    header('HTTP/1.0 403 Forbidden');
	    header('Content-Type: text/plain');
	    echo "Forbidden\n";
	    exit;
	}
	
	$cache_info = [];
	
	if (function_exists('opcache_get_status')) {
	    $cache_info['opcache_get_status'] = opcache_get_status(false);
	}
	if (function_exists('apc_cache_info')) {
	    $cache_info['apc_cache_info_user'] = apc_cache_info("user", true);
	}
	if (function_exists('apc_sma_info')) {
	    $cache_info['apc_sma_info'] = apc_sma_info(true);
	}
	
	header('Content-Type: application/json');
	echo json_encode($cache_info);
	
	********************************************************************
	And this should output something like :
	********************************************************************
	{
	"opcache_get_status": {
		"opcache_enabled": true,
		"cache_full": true,
		"restart_pending": false,
		"restart_in_progress": false,
		"memory_usage": {
			"used_memory": 44909176,
			"free_memory": 22176648,
			"wasted_memory": 23040,
			"current_wasted_percentage": 0.034332275390625
		},
		"interned_strings_usage": {
			"buffer_size": 4194304,
			"used_memory": 4194296,
			"free_memory": 8,
			"number_of_strings": 37270
		},
		"opcache_statistics": {
			"num_cached_scripts": 2131,
			"num_cached_keys": 3907,
			"max_cached_keys": 3907,
			"hits": 17896,
			"start_time": 1468430984,
			"last_restart_time": 0,
			"oom_restarts": 0,
			"hash_restarts": 0,
			"manual_restarts": 0,
			"misses": 6928,
			"blacklist_misses": 0,
			"blacklist_miss_ratio": 0,
			"opcache_hit_rate": 72.091524331292
		}
	},
	"apc_cache_info_user": {
		"num_slots": 4099,
		"ttl": 0,
		"num_hits": 18198,
		"num_misses": 4161,
		"num_inserts": 4193,
		"num_entries": 4174,
		"num_expunges": 0,
		"start_time": 1468430984,
		"mem_size": 2738144,
		"file_upload_progress": 1,
		"memory_type": "mmap"
	},
	"apc_sma_info": {
		"num_seg": 1,
		"seg_size": 33554296,
		"avail_mem": 30649592
	}
}
*******************************************************************	
*/

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"io/ioutil"
)

const (
	URL                 = "http://localhost/cache-status.php"
	COLLECTION_INTERVAL = 15
	DEFAULT_TIMEOUT     = 10
)

type phpCache struct {
	OpcacheGetStatus phpCacheOpcacheGetStatus `json:"opcache_get_status"`
	ApcCacheInfoUser phpCacheApcCacheInfoUser `json:"apc_cache_info_user"`
	ApcSmaInfo       phpCacheApcSmaInfo       `json:"apc_sma_info"`
}
type phpCacheOpcacheGetStatus struct {
	CacheFull         bool                      `json:"cache_full"`
	OpcacheEnabled    bool                      `json:"opcache_enabled"`
	RestartInProgress bool                      `json:"restart_in_progress"`
	RestartPending    bool                      `json:"restart_pending"`
	MemoryUsage       phpCacheMemoryUsage       `json:"memory_usage"`
	OpcacheStatistics phpCacheOpcacheStatistics `json:"opcache_statistics"`
}
type phpCacheMemoryUsage struct {
	CurrentWastedPercentage float64 `json:"current_wasted_percentage"`
	FreeMemory              int     `json:"free_memory"`
	UsedMemory              int     `json:"used_memory"`
	WastedMemory            int     `json:"wasted_memory"`
}
type phpCacheOpcacheStatistics struct {
	BlacklistMissRatio int     `json:"blacklist_miss_ratio"`
	BlacklistMisses    int     `json:"blacklist_misses"`
	HashRestarts       int     `json:"hash_restarts"`
	Hits               int     `json:"hits"`
	LastRestartTime    int     `json:"last_restart_time"`
	ManualRestarts     int     `json:"manual_restarts"`
	MaxCachedKeys      int     `json:"max_cached_keys"`
	Misses             int     `json:"misses"`
	NumCachedKeys      int     `json:"num_cached_keys"`
	NumCachedScripts   int     `json:"num_cached_scripts"`
	OomRestarts        int     `json:"oom_restarts"`
	OpcacheHitRate     float64 `json:"opcache_hit_rate"`
	StartTime          int     `json:"start_time"`
}
type phpCacheApcCacheInfoUser struct {
	Expunges           int    `json:"expunges"`
	FileUploadProgress int    `json:"file_upload_progress"`
	MemSize            int    `json:"mem_size"`
	MemoryType         string `json:"memory_type"`
	NumEntries         int    `json:"num_entries"`
	NumHits            int    `json:"num_hits"`
	NumInserts         int    `json:"num_inserts"`
	NumMisses          int    `json:"num_misses"`
	NumSlots           int    `json:"num_slots"`
	StartTime          int    `json:"start_time"`
	TTL                int    `json:"ttl"`
}
type phpCacheApcSmaInfo struct {
	NumSeg   int `json:"num_seg"`
	SegSize  int `json:"seg_size"`
	AvailMem int `json:"avail_mem"`
}

var phpcachedMeta = map[string]MetricMeta{
	//memory usage
	"opcache.memused": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Memory used of the opcache.",
	},
	"php.opcache.memfree": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Memory free of the opcache.",
	},
	"opcache.memwasted": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Memory wasted of the opcache.",
	},
	"opcache.memwastedpct": {
		RateType: metadata.Gauge,
		Unit:     metadata.Pct,
		Desc:     "Percentage of the memory wasted/(wasted + used) of the opcache.",
	},
	"opcache.memusedpct": {
		RateType: metadata.Gauge,
		Unit:     metadata.Pct,
		Desc:     "Percentage of the memory used/(used + free) of the opcache.",
	},
	//opcache_statistics
	"opcache.scripts": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of caches scripts of the opcache.",
	},
	"opcache.items": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of cached keys of the opcache.",
	},
	"opcache.maxitems": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Maximum of cached keys of the opcache.",
	},
	"opcache.itemspct": {
		RateType: metadata.Gauge,
		Unit:     metadata.Pct,
		Desc:     "Percentage of the cached keys (number/maximum) of the opcache.",
	},
	"opcache.hits": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of hits of the opcache.",
	},
	"opcache.misses": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of misses of the opcache.",
	},
	"opcache.restarts_oom": {
		TagSet:   opentsdb.TagSet{"type": "oom"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of oom restarts of the opcache.",
	},
	"opcache.restarts_hash": {
		TagSet:   opentsdb.TagSet{"type": "hash"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of hash restarts of the opcache.",
	},
	"opcache.restarts_manual": {
		TagSet:   opentsdb.TagSet{"type": "manual"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of manual restarts of the opcache.",
	},
	//apc_cache_info_user
	"apcu.hits": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of hits of the apcu.",
	},
	"apcu.misses": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of misses of the apcu.",
	},
	"apcu.expunges": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of expunges of the apcu.",
	},
	"apcu.inserts": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of inserts of the apcu.",
	},
	"apcu.items": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of entries of the apcu.",
	},
	"apcu.memused": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Memory size of the apcu.",
	},
	//apc_sma_info
	"apcu.memfree": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Available memory of the apcu.",
	},
	"apcu.memtotal": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total memory (available + memory size) of the apcu.",
	},
	"apcu.memusedpct": {
		RateType: metadata.Gauge,
		Unit:     metadata.Pct,
		Desc:     "Percentage of the memory used (memory size/(available + memory size)) of the apcu.",
	},
}

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_php_cache_stats})
}

func c_php_cache_stats() (opentsdb.MultiDataPoint, error) {

	var md opentsdb.MultiDataPoint

	var client http.Client = http.Client{
		Timeout: time.Second * DEFAULT_TIMEOUT,
	}
	response, err := client.Get(URL)

	if err != nil {
		log.Fatal(err)
	} else {
		if response != nil {
			defer time.Sleep(time.Second * COLLECTION_INTERVAL)

			defer response.Body.Close()

			var php phpCache

			jsonDataFromHttp, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Fatal(err)
			}
			if err := json.Unmarshal([]byte(jsonDataFromHttp), &php); err != nil {
				log.Fatal(err)
			}

			var mu phpCacheMemoryUsage = php.OpcacheGetStatus.MemoryUsage
			addElementSameKeyAndElementName(&md, "opcache.memused", mu.UsedMemory)
			addElementSameKeyAndElementName(&md, "opcache.memfree", mu.FreeMemory)
			addElementSameKeyAndElementName(&md, "opcache.memwasted", mu.WastedMemory)
			addElementSameKeyAndElementName(&md, "opcache.memwastedpct", mu.CurrentWastedPercentage)

			if mu.UsedMemory > 0 || mu.FreeMemory > 0 {
				addElementSameKeyAndElementName(&md, "opcache.memusedpct", 100.0*float64(mu.UsedMemory)/float64(mu.UsedMemory+mu.FreeMemory))
			}

			var os = php.OpcacheGetStatus.OpcacheStatistics
			addElementSameKeyAndElementName(&md, "opcache.scripts", os.NumCachedScripts)
			addElementSameKeyAndElementName(&md, "opcache.items", os.NumCachedKeys)
			addElementSameKeyAndElementName(&md, "opcache.maxitems", os.MaxCachedKeys)

			if os.MaxCachedKeys > 0 {
				addElementSameKeyAndElementName(&md, "opcache.itemspct", 100.0*float64(os.NumCachedKeys)/float64(os.MaxCachedKeys))
			}

			addElementSameKeyAndElementName(&md, "opcache.hits", os.Hits)
			addElementSameKeyAndElementName(&md, "opcache.misses", os.Misses)
			addElement(&md, "opcache.restarts_oom", "opcache.restarts", os.OomRestarts)
			addElement(&md, "opcache.restarts_hash", "opcache.restarts", os.HashRestarts)
			addElement(&md, "opcache.restarts_manual", "opcache.restarts", os.ManualRestarts)

			var au = php.ApcCacheInfoUser
			addElementSameKeyAndElementName(&md, "apcu.hits", au.NumHits)
			addElementSameKeyAndElementName(&md, "apcu.misses", au.NumMisses)
			addElementSameKeyAndElementName(&md, "apcu.expunges", au.Expunges)
			addElementSameKeyAndElementName(&md, "apcu.inserts", au.NumInserts)
			addElementSameKeyAndElementName(&md, "apcu.items", au.NumEntries)
			addElementSameKeyAndElementName(&md, "apcu.memused", au.MemSize)

			var am = php.ApcSmaInfo
			addElementSameKeyAndElementName(&md, "apcu.memfree", am.AvailMem)
			addElementSameKeyAndElementName(&md, "apcu.memtotal", am.AvailMem+au.MemSize)
			if am.AvailMem > 0 || au.MemSize > 0 {
				addElementSameKeyAndElementName(&md, "apcu.memusedpct", 100.0*float64(au.MemSize)/float64(am.AvailMem+au.MemSize))
			}
		}
	}
	return md, nil
}

func addElement(md *opentsdb.MultiDataPoint, keyName string, elementName string, value interface{}) {
	var elementTagSet = phpcachedMeta[keyName].TagSet
	var elementRateType = phpcachedMeta[keyName].RateType
	var elementUnit = phpcachedMeta[keyName].Unit
	var elementDesc = phpcachedMeta[keyName].Desc

	Add(md, "php."+elementName, value, elementTagSet, elementRateType, elementUnit, elementDesc)
}

func addElementSameKeyAndElementName(md *opentsdb.MultiDataPoint, elementName string, value interface{}) {
	addElement(md, elementName, elementName, value)
}
