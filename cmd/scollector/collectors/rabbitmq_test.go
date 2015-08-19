package collectors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"bosun.org/opentsdb"
)

func TestRmqOverview(t *testing.T) {
	t.Parallel()
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testRmqOverviewJSON)
	}))
	defer ht.Close()

	md, err := rabbitmqOverview(ht.URL)
	if err != nil {
		t.Error(err)
	}
	for _, emd := range testRmqOverviewMD {
		if !mdContains(t, md, emd) {
			t.Errorf("md must contain %v", emd)
		}
	}
}

func TestRmqQueues(t *testing.T) {
	t.Parallel()
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testRmqQueuesJSON)
	}))
	defer ht.Close()

	md, err := rabbitmqQueues(ht.URL)
	if err != nil {
		t.Error(err)
	}
	for _, emd := range testRmqQueuesMD {
		if !mdContains(t, md, emd) {
			t.Errorf("md must contain: '%v'", emd)
		}
	}

}

func TestRmqNodes(t *testing.T) {
	t.Parallel()
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testRmqNodesJSON)
	}))
	defer ht.Close()

	md, err := rabbitmqNodes(ht.URL)
	if err != nil {
		t.Error(err)
	}
	for _, emd := range testRmqNodesMD {
		if !mdContains(t, md, emd) {
			t.Errorf("md must contain: '%v'", emd)
		}
	}
}

func mdContains(t *testing.T, md opentsdb.MultiDataPoint, e *opentsdb.DataPoint) bool {
	for _, d := range md {
		if d.Metric == e.Metric && d.Tags.Equal(e.Tags) {
			if reflect.DeepEqual(d.Value, e.Value) {
				return true
			}
			t.Errorf("values differ, got: %v, expected: %v",
				d.Value, e.Value)

		}
	}
	return false
}

var testRmqOverviewJSON = `
{
  "rabbitmq_version": "3.4.0",
  "cluster_name": "rabbit@rabbit1",
  "erlang_version": "R16B03-1",
  "erlang_full_version": "Erlang R16B03-1 (erts-5.10.4) [source] [64-bit] [smp:4:4] [async-threads:30] [hipe] [kernel-poll:true]",
  "message_stats": {
    "publish": 87902,
    "deliver_get": 87231,
    "deliver_no_ack": 87231
  },
  "queue_totals": {
    "messages": 0,
    "messages_ready": 0,
    "messages_unacknowledged": 0
  },
  "object_totals": {
    "channels": 2,
    "connections": 2,
    "consumers": 1,
    "exchanges": 8,
    "queues": 1
  },
  "node": "rabbit@rabbit1"
}
`
var testRmqOverviewMD = opentsdb.MultiDataPoint{
	&opentsdb.DataPoint{
		Metric:    "rabbitmq.overview.message_stats",
		Timestamp: 0,
		Value:     87231,
		Tags:      opentsdb.TagSet{"host": "rabbit1", "method": "deliver_get"},
	},
}

var testRmqQueuesJSON = `
[
{
  "arguments": {},
  "auto_delete": false,
  "backing_queue_status": {
    "avg_ack_egress_rate": 0,
    "avg_ack_ingress_rate": 0,
    "avg_egress_rate": 6.278889329693902e-16,
    "avg_ingress_rate": 5.174965294975286e-16,
    "delta": [
      "delta",
      "undefined",
      0,
      "undefined"
    ],
    "len": 0,
    "mirror_seen": 0,
    "mirror_senders": 1,
    "next_seq_id": 917,
    "q1": 0,
    "q2": 0,
    "q3": 0,
    "q4": 0,
    "target_ram_count": "infinity"
  },
  "consumer_utilisation": 1,
  "consumers": 1,
  "down_slave_nodes": "",
  "durable": false,
  "memory": 68512,
  "message_bytes": 0,
  "message_bytes_persistent": 0,
  "message_bytes_ram": 0,
  "message_bytes_ready": 0,
  "message_bytes_unacknowledged": 0,
  "message_stats": {
    "deliver_get": 90345,
    "deliver_no_ack": 90345,
    "publish": 91028
  },
  "messages": 0,
  "messages_persistent": 0,
  "messages_ram": 0,
  "messages_ready": 0,
  "messages_ready_ram": 0,
  "messages_unacknowledged": 0,
  "messages_unacknowledged_ram": 0,
  "name": "hello",
  "node": "rabbit@rabbit1",
  "slave_nodes": [
    "rabbit@rabbit2",
    "rabbit@rabbit3"
  ],
  "state": "running",
  "synchronised_slave_nodes": [
    "rabbit@rabbit2",
    "rabbit@rabbit3"
  ],
  "vhost": "/"
}
]
`

var testRmqQueuesMD = opentsdb.MultiDataPoint{
	&opentsdb.DataPoint{
		Metric:    "rabbitmq.queue.consumer_utilisation",
		Timestamp: 0,
		Value:     1.0,
		Tags:      opentsdb.TagSet{"host": "rabbit1", "queue": "hello", "vhost": "/"},
	},
	&opentsdb.DataPoint{
		Metric:    "rabbitmq.queue.slave_nodes",
		Timestamp: 0,
		Value:     2,
		Tags:      opentsdb.TagSet{"host": "rabbit1", "queue": "hello", "vhost": "/"},
	},
	&opentsdb.DataPoint{
		Metric:    "rabbitmq.queue.state",
		Timestamp: 0,
		Value:     0,
		Tags:      opentsdb.TagSet{"host": "rabbit1", "queue": "hello", "vhost": "/"},
	},
}

var testRmqNodesJSON = `
[
{
  "disk_free": 3393945600,
  "disk_free_alarm": false,
  "disk_free_limit": 50000000,
  "fd_total": 1048576,
  "fd_used": 29,
  "mem_alarm": false,
  "mem_limit": 6639666790,
  "mem_used": 129488008,
  "name": "rabbit@rabbit1",
  "net_ticktime": 60,
  "os_pid": "13",
  "partitions": [
     ["rabbit1","rabbit2"],
     ["rabbit4","rabbit5"]
  ],
  "proc_total": 1048576,
  "proc_used": 233,
  "processors": 4,
  "rates_mode": "basic",
  "run_queue": 0,
  "running": true,
  "sockets_total": 943626,
  "sockets_used": 5,
  "type": "disc",
  "uptime": 305499
}
]
`

var testRmqNodesMD = opentsdb.MultiDataPoint{
	&opentsdb.DataPoint{
		Metric:    "rabbitmq.node.partitions",
		Timestamp: 0,
		Value:     2,
		Tags:      opentsdb.TagSet{"host": "rabbit1"},
	},
}
