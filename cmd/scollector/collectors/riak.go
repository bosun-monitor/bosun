package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

var riakMeta = map[string]MetricMeta{
	"pbc_connects_total": {
		Metric:   "pbc_connections",
		RateType: metadata.Counter,
		Unit:     metadata.Connection,
		Desc:     "Total number of Protocol Buffers connections made.",
	},
	"read_repairs_total": {
		Metric:   "read_repairs",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of Read Repairs this node has coordinated.",
	},
	"read_repairs_primary_outofdate_count": {
		Metric:   "read_repairs_primary_outofdate",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of read repair operations performed on primary vnodes due to stale replicas.",
	},
	"read_repairs_primary_notfound_count": {
		Metric:   "read_repairs_primary_notfound",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of read repair operations performed on primary vnodes due to missing replicas.",
	},
	"read_repairs_fallback_outofdate_count": {
		Metric:   "read_repairs_fallback_outofdate",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of read repair operations performed on fallback vnodes due to stale replicas.",
	},
	"read_repairs_fallback_notfound_count": {
		Metric:   "read_repairs_fallback_notfound",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of read repair operations performed on fallback vnodes due to missing replicas.",
	},
	"coord_redirs_total": {
		Metric:   "coord_redirs",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of requests this node has redirected to other nodes for coordination.",
	},
	"precommit_fail": {
		Metric:   "precommit_fail",
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Total number of pre-commit hook failures.",
	},
	"postcommit_fail": {
		Metric:   "postcommit_fail",
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Total number of post-commit hook failures.",
	},
	"executing_mappers": {
		Metric:   "executing_mappers",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
	},
	"pipeline_create_count": {
		Metric:   "pipeline.create.count",
		RateType: metadata.Counter,
		Unit:     metadata.Process,
		Desc:     "The total number of pipelines created since the node was started.",
	},
	"pipeline_create_error_count": {
		Metric:   "pipeline.create.errors",
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "The total number of pipeline creation errors since the node was started.",
	},
	"pipeline_active": {
		Metric:   "active",
		TagSet:   opentsdb.TagSet{"type": "pbc"},
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "The number of pipelines active in the last 60 seconds.",
	},
	"index_fsm_active": {
		Metric:   "active",
		TagSet:   opentsdb.TagSet{"type": "index"},
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Number of active Secondary Index FSMs.",
	},
	"list_fsm_active": {
		Metric:   "active",
		TagSet:   opentsdb.TagSet{"type": "list"},
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Number of active Keylisting FSMs.",
	},

	"memory_total": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "total"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total allocated memory (sum of processes and system).",
	},
	"memory_processes": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "processes"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory allocated for Erlang processes.",
	},
	"memory_processes_used": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "processes_used"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory used by Erlang processes.",
	},
	"memory_system": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "system"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total allocated memory that is not directly related to an Erlang process.",
	},
	"memory_system_used": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "system_used"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
	},
	"memory_atom": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "atom"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory currently allocated for atom storage.",
	},
	"memory_atom_used": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "atom_used"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory currently used for atom storage.",
	},
	"memory_binary": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "binary"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory used for binaries.",
	},
	"memory_code": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "code"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total amount of memory allocated for Erlang code.",
	},
	"memory_ets": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "ets"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total memory allocated for Erlang Term Storage.",
	},
	"mem_total": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "available"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total available system memory.",
	},
	"mem_allocated": {
		Metric:   "memory",
		TagSet:   opentsdb.TagSet{"type": "allocated"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total memory allocated for this node.",
	},

	"vnode_index_reads_total": {
		Metric:   "vnode.index.requests",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of local replicas participating in secondary index reads.",
	},
	"vnode_index_writes_total": {
		Metric:   "vnode.index.requests",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of local replicas participating in secondary index writes.",
	},
	"vnode_index_deletes_total": {
		Metric:   "vnode.index.requests",
		TagSet:   opentsdb.TagSet{"type": "delete"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of local replicas participating in secondary index deletes.",
	},
	"vnode_index_writes_postings_total": {
		Metric:   "vnode.index.requests",
		TagSet:   opentsdb.TagSet{"type": "write_post"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of individual secondary index values written.",
	},
	"vnode_index_deletes_postings_total": {
		Metric:   "vnode.index.requests",
		TagSet:   opentsdb.TagSet{"type": "delete_post"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of individual secondary index values deleted.",
	},

	"vnode_gets_total": {
		Metric:   "vnode.requests",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of GETs coordinated by local vnodes.",
	},
	"vnode_puts_total": {
		Metric:   "vnode.requests",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of PUTS coordinated by local vnodes.",
	},
	"node_gets_total": {
		Metric:   "node.requests",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of GETs coordinated by this node, including GETs to non-local vnodes.",
	},
	"node_puts_total": {
		Metric:   "node.requests",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of PUTs coordinated by this node, including PUTs to non-local vnodes.",
	},
	"node_get_fsm_time_mean": {
		Metric:   "node.latency.mean",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Mean time between reception of client GET request and subsequent response to client.",
	},
	"node_put_fsm_time_mean": {
		Metric:   "node.latency.mean",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Mean time between reception of client PUT request and subsequent response to client.",
	},
	"node_get_fsm_time_median": {
		Metric:   "node.latency.median",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Median time between reception of client GET request and subsequent response to client.",
	},
	"node_put_fsm_time_median": {
		Metric:   "node.latency.median",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Median time between reception of client PUT request and subsequent response to client.",
	},
	"node_get_fsm_time_95": {
		Metric:   "node.latency.95th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "95th percentile time between reception of client GET request and subsequent response to client.",
	},
	"node_put_fsm_time_95": {
		Metric:   "node.latency.95th",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "95th percentile time between reception of client PUT request and subsequent response to client.",
	},
	"node_get_fsm_time_99": {
		Metric:   "node.latency.99th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "99th percentile time between reception of client GET request and subsequent response to client.",
	},
	"node_put_fsm_time_99": {
		Metric:   "node.latency.99th",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "99th percentile time between reception of client PUT request and subsequent response to client.",
	},
	"node_get_fsm_time_100": {
		Metric:   "node.latency.100th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "100th percentile time between reception of client GET request and subsequent response to client.",
	},
	"node_put_fsm_time_100": {
		Metric:   "node.latency.100th",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "100th percentile time between reception of client PUT request and subsequent response to client.",
	},
	"node_get_fsm_objsize_mean": {
		Metric:   "node.objsize.mean",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Mean object size encountered by this node within the last minute.",
	},
	"node_get_fsm_objsize_median": {
		Metric:   "node.objsize.median",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Median object size encountered by this node within the last minute.",
	},
	"node_get_fsm_objsize_95": {
		Metric:   "node.objsize.95th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "95th percentile object size encountered by this node within the last minute.",
	},
	"node_get_fsm_objsize_99": {
		Metric:   "node.objsize.99th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "99th percentile object size encountered by this node within the last minute.",
	},
	"node_get_fsm_objsize_100": {
		Metric:   "node.objsize.100th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "100th percentile object size encountered by this node within the last minute.",
	},
	"node_get_fsm_siblings_mean": {
		Metric:   "node.siblings.mean",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Mean number of siblings encountered during all GET operations by this node within the last minute.",
	},
	"node_get_fsm_siblings_median": {
		Metric:   "node.siblings.median",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Median number of siblings encountered during all GET operations by this node within the last minute.",
	},
	"node_get_fsm_siblings_95": {
		Metric:   "node.siblings.95th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "95th percentile of siblings encountered during all GET operations by this node within the last minute.",
	},
	"node_get_fsm_siblings_99": {
		Metric:   "node.siblings.99th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "99th percentile of siblings encountered during all GET operations by this node within the last minute.",
	},
	"node_get_fsm_siblings_100": {
		Metric:   "node.siblings.100th",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "100th percentile of siblings encountered during all GET operations by this node within the last minute.",
	},
	"node_get_fsm_rejected_total": {
		Metric:   "node.requests.rejected",
		TagSet:   opentsdb.TagSet{"type": "get"},
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Total number of GET FSMs rejected by Sidejob's overload protection.",
	},
	"node_put_fsm_rejected_total": {
		Metric:   "node.requests.rejected",
		TagSet:   opentsdb.TagSet{"type": "put"},
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Total number of PUT FSMs rejected by Sidejob's overload protection.",
	},
	"ring_num_partitions": {
		Metric:   "ring_num_partitions",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "The number of partitions in the ring.",
	},
	"ring_creation_size": {
		Metric:   "ring.creation_size",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Ring size this cluster was created with.",
	},
	"cpu_nprocs": {
		Metric:   "cpu.nprocs",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Number of operating system processes.",
	},
	"cpu_avg1": {
		Metric:   "cpu.avg1",
		RateType: metadata.Gauge,
		Unit:     metadata.Load,
		Desc:     "The average number of active processes for the last 1 minute (equivalent to top(1) command’s load average when divided by 256()).",
	},
	"cpu_avg5": {
		Metric:   "cpu.avg5",
		RateType: metadata.Gauge,
		Unit:     metadata.Load,
		Desc:     "The average number of active processes for the last 5 minutes (equivalent to top(1) command’s load average when divided by 256()).",
	},
	"cpu_avg15": {
		Metric:   "cpu.avg15",
		RateType: metadata.Gauge,
		Unit:     metadata.Load,
		Desc:     "The average number of active processes for the last 15 minutes (equivalent to top(1) command’s load average when divided by 256()).",
	},
	"riak_search_vnodeq_total": {
		Metric:   "search.vnodeq",
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Total number of unprocessed messages all vnode message queues in the Riak Search subsystem have received on this node since it was started.",
	},
	"riak_search_vnodes_running": {
		Metric:   "search.vnodes_running",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Total number of vnodes currently running in the Riak Search subsystem.",
	},
}

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_riak, Enable: enableRiak})
}

const (
	localRiakURL string = "http://localhost:8098/stats"
)

func Riak(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return riak(s)
			},
			name: fmt.Sprintf("riak-%s", u.Host),
		})
	return nil
}

func enableRiak() bool {
	return enableURL(localRiakURL)()
}

func c_riak() (opentsdb.MultiDataPoint, error) {
	return riak(localRiakURL)
}

func riak(s string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	res, err := http.Get(s)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}
	for k, v := range r {
		if m, ok := riakMeta[k]; ok {
			if strings.HasPrefix(m.Metric, "node.latency") {
				if nl, ok := v.(float64); ok {
					v = nl / 1000000
				} else {
					err := fmt.Errorf("riak: bad integer %s in metric '%s'", v, m.Metric)
					return nil, err
				}
			}
			Add(&md, "riak."+m.Metric, v, m.TagSet, m.RateType, m.Unit, m.Desc)
		} else if k == "connected_nodes" {
			nodes, ok := v.([]interface{})
			// 'connected_nodes' array can be empty
			if !ok {
				err := fmt.Errorf("riak: unexpected content or type for 'connected_nodes' metric array")
				return nil, err
			}
			Add(&md, "riak.connected_nodes", len(nodes), nil, metadata.Gauge, metadata.Count, descConNodes)
		} else if k == "ring_members" {
			ringMembers, ok := v.([]interface{})
			// at least one ring member must always exist
			if !ok || len(ringMembers) < 1 {
				err := fmt.Errorf("riak: unexpected content or type for 'ring_members' metric array")
				return nil, err
			}
			Add(&md, "riak.ring_members", len(ringMembers), nil, metadata.Gauge, metadata.Count, descRingMembers)
		}
	}
	return md, nil
}

const (
	descConNodes    = "Count of nodes that this node is aware of at this time."
	descRingMembers = "Count of nodes that are members of the ring."
)
