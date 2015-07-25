package collectors

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"bytes"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

type MetricMetaFunc struct {
	ConvertFunc func(interface{}) (float64, error)
}

const (
	defaultRabbitmqURL string = "http://guest:guest@localhost:15672"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_rabbitmq_overview, Enable: enableRabbitmq})
	collectors = append(collectors, &IntervalCollector{F: c_rabbitmq_queues, Enable: enableRabbitmq})
	collectors = append(collectors, &IntervalCollector{F: c_rabbitmq_nodes, Enable: enableRabbitmq})
}

// RabbitMQ registers a RabbitMQ collector.
func RabbitMQ(url string) error {
	safeURL, err := urlUserHost(url)
	if err != nil {
		return err
	}
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return rabbitmqOverview(url)
			},
			name: fmt.Sprintf("rabbitmq-overview-%s", safeURL),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return rabbitmqNodes(url)
			},
			name: fmt.Sprintf("rabbitmq-nodes-%s", safeURL),
		},
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return rabbitmqQueues(url)
			},
			name: fmt.Sprintf("rabbitmq-queues-%s", safeURL),
		})
	return nil
}

func enableRabbitmq() bool {
	return enableURL(defaultRabbitmqURL)()
}

func c_rabbitmq_overview() (opentsdb.MultiDataPoint, error) {
	return rabbitmqOverview(defaultRabbitmqURL)

}
func c_rabbitmq_nodes() (opentsdb.MultiDataPoint, error) {
	return rabbitmqNodes(defaultRabbitmqURL)
}
func c_rabbitmq_queues() (opentsdb.MultiDataPoint, error) {
	return rabbitmqQueues(defaultRabbitmqURL)
}

func rabbitmqOverview(url string) (opentsdb.MultiDataPoint, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Get(url + "/api/overview")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var overview map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(content[:])).Decode(&overview); err != nil {
		return nil, err
	}
	flatOverview, err := explodeMap(overview, "", ".")
	if err != nil {
		return nil, err
	}
	node, ok := flatOverview["node"].(string)
	if !ok {
		err := fmt.Errorf("rabbitmq: invalid node name '%s'", node)
		return nil, err
	}
	host := strings.Split(node, "@")[1]
	tagset := opentsdb.TagSet{"host": host}
	var md opentsdb.MultiDataPoint
	for k, v := range flatOverview {
		if m, ok := rabbitmqMeta[k]; ok {
			val, ok := v.(float64)
			if !ok {
				err := fmt.Errorf("rabbitmq: unexpected content or type for '%s' key, got '%v'", k, v)
				return nil, err

			}
			Add(&md, "rabbitmq.overview."+m.Metric, val, tagset.Copy().Merge(m.TagSet), m.RateType, m.Unit, m.Desc)
		}
	}
	return md, nil
}

func rabbitmqQueues(url string) (opentsdb.MultiDataPoint, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	queueRes, err := client.Get(url + "/api/queues")
	if err != nil {
		return nil, err
	}
	defer queueRes.Body.Close()
	queueContent, err := ioutil.ReadAll(queueRes.Body)
	if err != nil {
		return nil, err
	}
	var queues []map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(queueContent[:])).Decode(&queues); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, qv := range queues {
		queueName, ok := qv["name"].(string)
		if !ok {
			err := fmt.Errorf("rabbitmq: invalid queue name '%s'", queueName)
			return nil, err
		}
		if strings.HasPrefix(queueName, "amq.gen") {
			// skip generated queues
			continue
		}
		vhost, ok := qv["vhost"].(string)
		if !ok {
			err := fmt.Errorf("rabbitmq: invalid vhost name '%s'", vhost)
			return nil, err
		}
		node, ok := qv["node"].(string)
		if !ok {
			err := fmt.Errorf("rabbitmq: invalid node name '%s'", node)
			return nil, err
		}
		host := strings.Split(node, "@")[1]
		tagset := opentsdb.TagSet{
			"host":  host,
			"queue": queueName,
			"vhost": vhost,
		}
		flatQueue, err := explodeMap(qv, "", ".")
		if err != nil {
			return nil, err
		}
		for k, v := range flatQueue {
			if m, ok := rabbitmqMeta[k]; ok {
				if rabbitmqNodesMetaFunc[k].ConvertFunc == nil {
					val, ok := v.(float64)
					if !ok {
						err := fmt.Errorf("rabbitmq: unexpected content or type for '%s' key, got '%v'", k, v)
						return nil, err

					}
					Add(&md, "rabbitmq.queues."+m.Metric, val,
						tagset.Copy().Merge(m.TagSet),
						m.RateType, m.Unit, m.Desc)
				} else {
					val, err := rabbitmqNodesMetaFunc[k].ConvertFunc(v)
					if err != nil {
						return nil, err

					}
					Add(&md, "rabbitmq.queues."+m.Metric, val,
						tagset.Copy().Merge(m.TagSet),
						m.RateType, m.Unit, m.Desc)
				}

			}
		}
	}
	return md, nil
}

func rabbitmqNodes(url string) (opentsdb.MultiDataPoint, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Get(url + "/api/nodes")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var nodes []map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(content[:])).Decode(&nodes); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, nv := range nodes {
		name, ok := nv["name"].(string)
		if !ok {
			err := fmt.Errorf("rabbitmq: invalid node name '%s'", name)
			return nil, err
		}
		host := strings.Split(name, "@")[1]
		tagset := opentsdb.TagSet{"host": host}
		flatNode, err := explodeMap(nv, "", ".")
		if err != nil {
			return nil, err
		}
		for k, v := range flatNode {
			if m, ok := rabbitmqMeta[k]; ok {
				if rabbitmqNodesMetaFunc[k].ConvertFunc == nil {
					val, ok := v.(float64)
					if !ok {
						err := fmt.Errorf("rabbitmq: unexpected content or type for '%s' key", k)
						return nil, err

					}
					Add(&md, "rabbitmq.nodes."+m.Metric, val,
						tagset.Copy().Merge(m.TagSet),
						m.RateType, m.Unit, m.Desc)
				} else {
					val, err := rabbitmqNodesMetaFunc[k].ConvertFunc(v)
					if err != nil {
						return nil, err

					}
					Add(&md, "rabbitmq.nodes."+m.Metric, val,
						tagset.Merge(m.TagSet),
						m.RateType, m.Unit, m.Desc)
				}
			}
		}
	}
	return md, nil
}

var rabbitmqMeta = map[string]MetricMeta{
	"object_totals.channels": {
		Metric:   "channels",
		RateType: metadata.Gauge,
		Unit:     metadata.Channel,
		Desc:     "Overall number of channels.",
	},
	"object_totals.connections": {
		Metric:   "connections",
		RateType: metadata.Gauge,
		Unit:     metadata.Connection,
		Desc:     "Overall number of connections.",
	},
	"object_totals.consumers": {
		Metric:   "consumers",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Overall number of consumers.",
	},
	"object_totals.exchanges": {
		Metric:   "exchanges",
		RateType: metadata.Gauge,
		Unit:     metadata.Exchange,
		Desc:     "Overall number of exchanges.",
	},
	"object_totals.queues": {
		Metric:   "queues",
		RateType: metadata.Gauge,
		Unit:     metadata.Queue,
		Desc:     "Overall number of queues.",
	},
	"queue_totals.messages": {
		Metric:   "messages_total",
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Overall sum of ready and unacknowledged messages (queue depth).",
	},
	"queue_totals.messages_ready": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "ready"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Overall number of messages ready to be delivered to clients.",
	},
	"queue_totals.messages_unacknowledged": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "unack"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Overall number of messages delivered to clients but not yet acknowledged.",
	},
	"message_stats.ack": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"state": "ack"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of acknowledged messages.",
	},
	"message_stats.publish": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "publish"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages published.",
	},
	"message_stats.publish_in": {
		Metric:   "message_stats_publish_in",
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages published \"in\" to an exchange, i.e. not taking account of routing.",
	},
	"message_stats.publish_out": {
		Metric:   "message_stats_publish_out",
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages published \"out\" of an exchange, i.e. taking account of routing.",
	},
	"message_stats.confirm": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "confirm"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages confirmed.",
	},
	"message_stats.deliver": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "deliver_ack"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages delivered in acknowledgement mode to consumers.",
	},
	"message_stats.deliver_no_ack": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "deliver_noack"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages delivered in no-acknowledgement mode to consumers.",
	},
	"message_stats.get": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "get_ack"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages delivered in acknowledgement mode in response to basic.get.",
	},
	"message_stats.get_noack": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "get_noack"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages delivered in no-acknowledgement mode in response to basic.get.",
	},
	"message_stats.deliver_get": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "deliver_get"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Sum of deliver, deliver_noack, get, get_noack.",
	},
	"message_stats.redeliver": {
		Metric:   "message_stats",
		TagSet:   opentsdb.TagSet{"method": "redeliver"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of subset of messages in deliver_get which had the redelivered flag set.",
	},
	"message_stats.return": {
		Metric:   "message_stats_return",
		TagSet:   opentsdb.TagSet{"method": "return"},
		RateType: metadata.Counter,
		Unit:     metadata.Message,
		Desc:     "Count of messages returned to publisher as unroutable.",
	},
	"memory": {
		Metric:   "memory",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Bytes of memory consumed by the Erlang process associated with the queue, including stack, heap and internal structures.",
	},
	"consumers": {
		Metric:   "consumers",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Number of consumers.",
	},
	"messages": {
		Metric:   "messages_queue_depth",
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Sum of ready and unacknowledged messages (queue depth).",
	},
	"messages_ram": {
		Metric:   "messages_total",
		TagSet:   opentsdb.TagSet{"store": "transient"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Total number of messages which are resident in ram.",
	},
	"messages_persistent": {
		Metric:   "messages_total",
		TagSet:   opentsdb.TagSet{"store": "persistent"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Total number of persistent messages in the queue (will always be 0 for transient queues).",
	},
	"messages_ready": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "ready"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages ready to be delivered to clients.",
	},
	"messages_unacknowledged": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "unack"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages delivered to clients but not yet acknowledged.",
	},
	"messages_ready_ram": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "ready", "store": "transient"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages from messages_ready which are resident in ram.",
	},
	"messages_unacknowledged_ram": {
		Metric:   "messages",
		TagSet:   opentsdb.TagSet{"state": "unack", "store": "transient"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages from messages_unacknowledged which are resident in ram.",
	},
	"message_bytes": {
		Metric:   "message_bytes_total",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Sum of the size of all message bodies in the queue. This does not include the message properties (including headers) or any overhead.",
	},
	"message_bytes_ready": {
		Metric:   "message_bytes_ready",
		TagSet:   opentsdb.TagSet{"state": "ready"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Like message_bytes but counting only those messages ready to be delivered to clients.",
	},
	"message_bytes_unacknowledged": {
		Metric:   "message_bytes",
		TagSet:   opentsdb.TagSet{"state": "unack"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Like message_bytes but counting only those messages delivered to clients but not yet acknowledged.",
	},
	"message_bytes_persistent": {
		Metric:   "message_bytes",
		TagSet:   opentsdb.TagSet{"store": "persistent"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Like message_bytes but counting only those messages which are persistent.",
	},
	"message_bytes_ram": {
		Metric:   "message_bytes",
		TagSet:   opentsdb.TagSet{"store": "transient"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Like message_bytes but counting only those messages which are in RAM.",
	},
	"mem_used": {
		Metric:   "mem_used",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Memory used in bytes.",
	},
	"mem_limit": {
		Metric:   "mem_limit",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Point at which the memory alarm will go off.",
	},
	"disk_free": {
		Metric:   "disk_free",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Disk free space in bytes.",
	},
	"disk_free_limit": {
		Metric:   "disk_free_limit",
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Point at which the disk alarm will go off.",
	},
	"fd_total": {
		Metric:   "fd_total",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "File descriptors available.",
	},
	"fd_used": {
		Metric:   "fd_used",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Used file descriptors.",
	},
	"io_read_avg_time": {
		Metric:   "io_avg_time",
		TagSet:   opentsdb.TagSet{"type": "read"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Average wall time (milliseconds) for each disk read operation in the last statistics interval.",
	},
	"io_seek_avg_time": {
		Metric:   "io_avg_time",
		TagSet:   opentsdb.TagSet{"type": "seek"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Average wall time (milliseconds) for each seek operation in the last statistics interval.",
	},
	"io_sync_avg_time": {
		Metric:   "io_avg_time",
		TagSet:   opentsdb.TagSet{"type": "sync"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Average wall time (milliseconds) for each sync operation in the last statistics interval.",
	},
	"io_write_avg_time": {
		Metric:   "io_avg_time",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Average wall time (milliseconds) for each write operation in the last statistics interval.",
	},
	"io_read_bytes": {
		Metric:   "io_bytes",
		TagSet:   opentsdb.TagSet{"type": "read"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total number of bytes read from disk by the persister.",
	},
	"io_write_bytes": {
		Metric:   "io_bytes",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Total number of bytes written to disk by the persister.",
	},

	"io_read_count": {
		Metric:   "io_count",
		TagSet:   opentsdb.TagSet{"type": "read"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of read operations by the persister.",
	},
	"io_seek_count": {
		Metric:   "io_count",
		TagSet:   opentsdb.TagSet{"type": "seek"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of seek operations by the persister.",
	},
	"io_sync_count": {
		Metric:   "io_count",
		TagSet:   opentsdb.TagSet{"type": "sync"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of fsync() operations by the persister.",
	},
	"io_write_count": {
		Metric:   "io_count",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of write operations by the persister.",
	},
	"io_reopen_count": {
		Metric:   "io_count",
		TagSet:   opentsdb.TagSet{"type": "reopen"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of times the persister has needed to recycle file handles between queues. In an ideal world this number will be zero; if the number is large, performance might be improved by increasing the number of file handles available to RabbitMQ.",
	},
	"mnesia_disk_tx_count": {
		Metric:   "mnesia_tx_count",
		TagSet:   opentsdb.TagSet{"store": "persistent"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of Mnesia transactions which have been performed that required writes to disk. (e.g. creating a durable queue). Only transactions which originated on this node are included.",
	},
	"mnesia_ram_tx_count": {
		Metric:   "mnesia_tx_count",
		TagSet:   opentsdb.TagSet{"store": "transient"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of Mnesia transactions which have been performed that did not require writes to disk. (e.g. creating a transient queue). Only transactions which originated on this node are included.",
	},
	"msg_store_read_count": {
		Metric:   "mnesia_store_count",
		TagSet:   opentsdb.TagSet{"type": "read"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages which have been read from the message store.",
	},
	"msg_store_write_count": {
		Metric:   "mnesia_store_count",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Number of messages which have been written to the message store.",
	},
	"proc_total": {
		Metric:   "proc_total",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Maximum number of Erlang processes.",
	},
	"proc_used": {
		Metric:   "proc_used",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Number of Erlang processes in use.",
	},
	"processors": {
		Metric:   "processors",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Number of cores detected and usable by Erlang.",
	},
	"queue_index_journal_write_count": {
		Metric:   "queue_index_count",
		TagSet:   opentsdb.TagSet{"type": "journal_write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of records written to the queue index journal. Each record represents a message being published to a queue, being delivered from a queue, and being acknowledged in a queue.",
	},
	"queue_index_read_count": {
		Metric:   "queue_index_count",
		TagSet:   opentsdb.TagSet{"type": "read"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of records read from the queue index.",
	},
	"queue_index_write_count": {
		Metric:   "queue_index_count",
		TagSet:   opentsdb.TagSet{"type": "write"},
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of records written to the queue index.",
	},
	"run_queue": {
		Metric:   "run_queue",
		RateType: metadata.Gauge,
		Unit:     metadata.Process,
		Desc:     "Average number of Erlang processes waiting to run.",
	},
	"sockets_total": {
		Metric:   "sockets_total",
		RateType: metadata.Gauge,
		Unit:     metadata.Connection,
		Desc:     "File descriptors available for use as sockets.",
	},
	"sockets_used": {
		Metric:   "sockets_used",
		RateType: metadata.Gauge,
		Unit:     metadata.Connection,
		Desc:     "File descriptors used as sockets.",
	},
	"partitions": {
		Metric:   "partitions",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Count of network partitions this node is seeing.",
	},
	"mem_alarm": {
		Metric:   "mem_alarm",
		RateType: metadata.Gauge,
		Unit:     metadata.Bool,
		Desc:     "",
	},
	"disk_free_alarm": {
		Metric:   "disk_free_alarm",
		RateType: metadata.Gauge,
		Unit:     metadata.Bool,
		Desc:     "Whether the disk alarm has gone off.",
	},
	// backing queue metrics descriptions taken from
	// github.com/michaelklishin/rabbit-hole
	"backing_queue_status.len": {
		Metric:   "backing_queue_len",
		RateType: metadata.Gauge,
		Unit:     metadata.Message,
		Desc:     "Total backing queue length.",
	},
	"backing_queue_status.avg_ingress_rate": {
		Metric:   "backing_queue_rate",
		TagSet:   opentsdb.TagSet{"direction": "in"},
		RateType: metadata.Gauge,
		Unit:     metadata.Rate,
		Desc:     "Average ingress (inbound) rate, not including messages that straight through to auto-acking consumers.",
	},
	"backing_queue_status.avg_egress_rate": {
		Metric:   "backing_queue_rate",
		TagSet:   opentsdb.TagSet{"direction": "out"},
		RateType: metadata.Gauge,
		Unit:     metadata.Rate,
		Desc:     "Average egress (outbound) rate, not including messages that straight through to auto-acking consumers.",
	},
	"backing_queue_status.avg_ack_ingress_rate": {
		Metric:   "backing_queue_ack_rate",
		TagSet:   opentsdb.TagSet{"direction": "in"},
		RateType: metadata.Gauge,
		Unit:     metadata.Rate,
		Desc:     "Rate at which unacknowledged message records enter RAM, e.g. because messages are delivered requiring acknowledgement",
	},
	"backing_queue_status.avg_ack_egress_rate": {
		Metric:   "backing_queue_ack_rate",
		TagSet:   opentsdb.TagSet{"direction": "out"},
		RateType: metadata.Gauge,
		Unit:     metadata.Rate,
		Desc:     "Rate at which unacknowledged message records leave RAM, e.g. because acks arrive or unacked messages are paged out",
	},
	"state": {
		Metric:   "state",
		RateType: metadata.Gauge,
		Unit:     metadata.StatusCode,
		Desc:     "The state of the queue. Unknown: 0, Running: 1, Syncing: 2, Flow: 3, Down: 4",
	},
	"running": {
		Metric:   "running",
		RateType: metadata.Gauge,
		Unit:     metadata.Bool,
		Desc:     "Boolean for whether this node is up. Obviously if this is false, most other stats will be missing.",
	},
	"uptime": {
		Metric:   "uptime",
		RateType: metadata.Counter,
		Unit:     metadata.Second,
		Desc:     "Node uptime in seconds.",
	},
}

var rabbitmqNodesMetaFunc = map[string]MetricMetaFunc{
	"partitions": {
		ConvertFunc: sliceSizeToMetric,
	},
	"mem_alarm": {
		ConvertFunc: boolToMetric,
	},
	"disk_free_alarm": {
		ConvertFunc: boolToMetric,
	},
	"running": {
		ConvertFunc: boolToMetric,
	},
	"state": {
		ConvertFunc: rabbitmqQueueState,
	},
}

func rabbitmqQueueState(i interface{}) (float64, error) {
	s, ok := i.(string)
	if !ok {
		err := fmt.Errorf("unexpected size or type of queue state metric, got: '%v'", i)
		return 0.0, err
	}
	if s == "running" {
		return 1.0, nil
	} else if s == "syncing" { // TODO: find proper format
		return 2.0, nil
	} else if s == "flow" {
		return 3.0, nil
	} else if s == "idle" {
		return 4.0, nil
	} else if s == "down" {
		return 5.0, nil
	}
	slog.Infof("rabbitmq: unknown state metric value: '%v'", i)
	return 0.0, nil
}

func sliceSizeToMetric(i interface{}) (float64, error) {
	switch v := i.(type) {
	case []string:
		return float64(len(v)), nil
	case []interface{}:
		return float64(len(v)), nil
	default:
		err := fmt.Errorf("unexpected size or type of metric array")
		return 0, err
	}
}

func boolToMetric(i interface{}) (float64, error) {
	b, ok := i.(bool)
	if !ok {
		err := fmt.Errorf("unexpected size or type of boolean metric, got: '%v'", i)
		return 0, err
	}
	if b == true {
		return 1.0, nil
	}
	return 0.0, nil
}

func urlUserHost(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	res := fmt.Sprintf("%s@%s", u.User.Username(), u.Host)
	return res, nil
}

func explodeMap(m map[string]interface{}, parent string, delimiter string) (map[string]interface{}, error) {
	var err error
	j := make(map[string]interface{})
	for k, i := range m {
		if len(parent) > 0 {
			k = parent + delimiter + k
		}
		switch v := i.(type) {
		case nil:
			j[k] = v
		case int:
			j[k] = v
		case float64:
			j[k] = v
		case string:
			j[k] = v
		case bool:
			j[k] = v
		case []interface{}:
			j[k] = v
		case map[string]interface{}:
			out := make(map[string]interface{})
			out, err = explodeMap(v, k, delimiter)
			if err != nil {
				return nil, err
			}
			for key, value := range out {
				j[key] = value
			}
		default:
			//nothing
		}
	}
	return j, nil
}
