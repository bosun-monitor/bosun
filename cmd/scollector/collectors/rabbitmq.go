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

var (
	rmqQueueStatusMap = map[string]int{
		"running": 0,
		"syncing": 1,
		"flow":    2,
		"idle":    3,
		"down":    4,
	}
)

const (
	defaultRabbitmqURL string = "http://guest:guest@localhost:15672"
	rmqPrefix                 = "rabbitmq."
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: cRabbitmqOverview, Enable: enableRabbitmq})
	collectors = append(collectors, &IntervalCollector{F: cRabbitmqQueues, Enable: enableRabbitmq})
	collectors = append(collectors, &IntervalCollector{F: cRabbitmqNodes, Enable: enableRabbitmq})
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

func cRabbitmqOverview() (opentsdb.MultiDataPoint, error) {
	return rabbitmqOverview(defaultRabbitmqURL)

}
func cRabbitmqNodes() (opentsdb.MultiDataPoint, error) {
	return rabbitmqNodes(defaultRabbitmqURL)
}
func cRabbitmqQueues() (opentsdb.MultiDataPoint, error) {
	return rabbitmqQueues(defaultRabbitmqURL)
}

func rabbitmqOverview(s string) (opentsdb.MultiDataPoint, error) {
	p := rmqPrefix + "overview."
	res, err := http.Get(s + "/api/overview")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var o rmqOverview
	if err := json.NewDecoder(res.Body).Decode(&o); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	splitNode := strings.Split(o.Node, "@")
	if len(splitNode) < 2 {
		return nil, fmt.Errorf("Error: invalid RabbitMQ node name, can not split '%s'", o.Node)
	}
	host := splitNode[1]
	ts := opentsdb.TagSet{"host": host}
	Add(&md, p+"channels", o.ObjectTotals.Channels, ts, metadata.Gauge, metadata.Channel, DescRmqObjecttotalsChannels)
	Add(&md, p+"connections", o.ObjectTotals.Connections, ts, metadata.Gauge, metadata.Connection, DescRmqObjectTotalsConnections)
	Add(&md, p+"consumers", o.ObjectTotals.Consumers, ts, metadata.Gauge, metadata.Consumer, DescRmqObjectTotalsConsumers)
	Add(&md, p+"exchanges", o.ObjectTotals.Exchanges, ts, metadata.Gauge, metadata.Exchange, DescRmqObjectTotalsExchanges)
	Add(&md, p+"queues", o.ObjectTotals.Queues, ts, metadata.Gauge, metadata.Queue, DescRmqObjectTotalsQueues)
	msgStats := rabbitmqMessageStats(p, ts, o.MessageStats)
	md = append(md, msgStats...)
	return md, nil
}

func rabbitmqQueues(s string) (opentsdb.MultiDataPoint, error) {
	p := rmqPrefix + "queue."
	res, err := http.Get(s + "/api/queues")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var qs []rmqQueue
	if err := json.NewDecoder(res.Body).Decode(&qs); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, q := range qs {
		if strings.HasPrefix(q.Name, "amq.gen-") {
			continue // skip auto-generated queues
		}
		splitNode := strings.Split(q.Node, "@")
		if len(splitNode) < 2 {
			return nil, fmt.Errorf("Error: invalid RabbitMQ node name, can not split '%s'", q.Node)
		}
		host := splitNode[1]
		ts := opentsdb.TagSet{"host": host, "queue": q.Name, "vhost": q.Vhost}
		Add(&md, p+"consumers", q.Consumers, ts, metadata.Gauge, metadata.Consumer, DescRmqConsumers)
		Add(&md, p+"memory", q.Memory, ts, metadata.Gauge, metadata.Bytes, DescRmqMemory)
		Add(&md, p+"message_bytes_total", q.MessageBytes, ts, metadata.Gauge, metadata.Bytes, DescRmqMessageBytes)
		Add(&md, p+"message_bytes_persistent", q.MessageBytesPersistent, ts, metadata.Gauge, metadata.Bytes, DescRmqMessageBytesPersistent)
		Add(&md, p+"message_bytes_transient", q.MessageBytesRAM, ts, metadata.Gauge, metadata.Bytes, DescRmqMessageBytesRAM)
		Add(&md, p+"message_bytes_ready", q.MessageBytesReady, ts, metadata.Gauge, metadata.Bytes, DescRmqMessageBytesReady)
		Add(&md, p+"message_bytes_unack", q.MessageBytesUnacknowledged, ts, metadata.Gauge, metadata.Bytes, DescRmqMessageBytesUnacknowledged)
		Add(&md, p+"messages_queue_depth", q.Messages, ts, metadata.Gauge, metadata.Message, DescRmqMessages)
		Add(&md, p+"messages_persistent", q.MessagesPersistent, ts, metadata.Gauge, metadata.Message, DescRmqMessagesPersistent)
		Add(&md, p+"messages_transient", q.MessagesRAM, ts, metadata.Gauge, metadata.Message, DescRmqMessagesRAM)
		Add(&md, p+"messages_ready_total", q.MessagesReady, ts, metadata.Gauge, metadata.Message, DescRmqMessagesReady)
		Add(&md, p+"messages_ready_transient", q.MessagesReadyRAM, ts, metadata.Gauge, metadata.Message, DescRmqMessagesReadyRAM)
		Add(&md, p+"messages_unack_total", q.MessagesUnacknowledged, ts, metadata.Gauge, metadata.Message, DescRmqMessagesUnacknowledged)
		Add(&md, p+"messages_unack_transient", q.MessagesUnacknowledgedRAM, ts, metadata.Gauge, metadata.Message, DescRmqMessagesUnacknowledgedRAM)
		if sn, ok := q.SlaveNodes.([]interface{}); ok {
			Add(&md, p+"slave_nodes", len(sn), ts, metadata.Gauge, metadata.Node, DescRmqSlaveNodes)
		}
		if dsn, ok := q.DownSlaveNodes.([]interface{}); ok {
			Add(&md, p+"down_slave_nodes", len(dsn), ts, metadata.Gauge, metadata.Node, DescRmqDownSlaveNodes)

		}
		if ssn, ok := q.SynchronisedSlaveNodes.([]interface{}); ok {
			Add(&md, p+"sync_slave_nodes", len(ssn), ts, metadata.Gauge, metadata.Node, DescRmqSynchronisedSlaveNodes)

		}
		if cu, ok := q.ConsumerUtilisation.(float64); ok {
			Add(&md, p+"consumer_utilisation", cu, ts, metadata.Gauge, metadata.Fraction, DescRmqConsumerUtilisation)
		}
		msgStats := rabbitmqMessageStats(p, ts, q.MessageStats)
		md = append(md, msgStats...)
		backingQueueStatus := rabbitmqBackingQueueStatus(p+"backing_queue.", ts, q.BackingQueueStatus)
		md = append(md, backingQueueStatus...)
		if state, ok := rmqQueueStatusMap[q.State]; ok {
			Add(&md, p+"state", state, ts, metadata.Gauge, metadata.StatusCode, DescRmqState)
		} else {
			Add(&md, p+"state", -1, ts, metadata.Gauge, metadata.StatusCode, DescRmqState)
		}
	}
	return md, nil
}
func rabbitmqNodes(s string) (opentsdb.MultiDataPoint, error) {
	p := rmqPrefix + "node."
	res, err := http.Get(s + "/api/nodes")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var ns []rmqNode
	if err := json.NewDecoder(res.Body).Decode(&ns); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, n := range ns {
		splitName := strings.Split(n.Name, "@")
		if len(splitName) < 2 {
			return nil, fmt.Errorf("Error: invalid RabbitMQ node name, can not split '%s'", n.Name)
		}
		host := splitName[1]
		ts := opentsdb.TagSet{"host": host}
		Add(&md, p+"disk_free", n.DiskFree, ts, metadata.Gauge, metadata.Consumer, DescRmqDiskFree)
		Add(&md, p+"disk_free_alarm", n.DiskFreeAlarm, ts, metadata.Gauge, metadata.Bool, DescRmqDiskFreeAlarm)
		Add(&md, p+"disk_free_limit", n.DiskFreeLimit, ts, metadata.Gauge, metadata.Consumer, DescRmqDiskFreeLimit)
		Add(&md, p+"fd_total", n.FDTotal, ts, metadata.Gauge, metadata.Files, DescRmqFDTotal)
		Add(&md, p+"fd_used", n.FDUsed, ts, metadata.Gauge, metadata.Files, DescRmqFDUsed)
		Add(&md, p+"mem_used", n.MemUsed, ts, metadata.Gauge, metadata.Bytes, DescRmqMemUsed)
		Add(&md, p+"mem_alarm", n.MemAlarm, ts, metadata.Gauge, metadata.Bool, DescRmqMemAlarm)
		Add(&md, p+"mem_limit", n.MemLimit, ts, metadata.Gauge, metadata.Bytes, DescRmqMemLimit)
		Add(&md, p+"proc_used", n.ProcUsed, ts, metadata.Gauge, metadata.Process, DescRmqProcUsed)
		Add(&md, p+"proc_total", n.ProcTotal, ts, metadata.Gauge, metadata.Process, DescRmqProcTotal)
		Add(&md, p+"sockets_used", n.SocketsUsed, ts, metadata.Gauge, metadata.Socket, DescRmqSocketsUsed)
		Add(&md, p+"sockets_total", n.SocketsTotal, ts, metadata.Gauge, metadata.Socket, DescRmqSocketsTotal)
		Add(&md, p+"uptime", n.Uptime, ts, metadata.Gauge, metadata.Second, DescRmqUptime)
		Add(&md, p+"running", n.Running, ts, metadata.Gauge, metadata.StatusCode, DescRmqRunning)
		if partitions, ok := n.Partitions.([]interface{}); ok {
			Add(&md, p+"partitions", len(partitions), ts, metadata.Gauge, metadata.Node, DescRmqPartitions)
		}

	}
	return md, nil
}

func rabbitmqMessageStats(p string, ts opentsdb.TagSet, ms rmqMessageStats) opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	Add(&md, p+"message_stats", ms.Ack, ts.Copy().Merge(opentsdb.TagSet{"method": "ack"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsAck)
	Add(&md, p+"message_stats", ms.Confirm, ts.Copy().Merge(opentsdb.TagSet{"method": "confirm"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsConfirm)
	Add(&md, p+"message_stats", ms.Deliver, ts.Copy().Merge(opentsdb.TagSet{"method": "deliver"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsDeliver)
	Add(&md, p+"message_stats", ms.DeliverGet, ts.Copy().Merge(opentsdb.TagSet{"method": "deliver_get"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsDeliverGet)
	Add(&md, p+"message_stats", ms.DeliverNoAck, ts.Copy().Merge(opentsdb.TagSet{"method": "deliver_noack"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsDeliverNoAck)
	Add(&md, p+"message_stats", ms.Get, ts.Copy().Merge(opentsdb.TagSet{"method": "get"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsGet)
	Add(&md, p+"message_stats", ms.GetNoAck, ts.Copy().Merge(opentsdb.TagSet{"method": "get_noack"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsGetNoack)
	Add(&md, p+"message_stats", ms.Publish, ts.Copy().Merge(opentsdb.TagSet{"method": "publish"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsPublish)
	Add(&md, p+"message_stats", ms.PublishIn, ts.Copy().Merge(opentsdb.TagSet{"method": "publish_in"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsPublishIn)
	Add(&md, p+"message_stats", ms.PublishOut, ts.Copy().Merge(opentsdb.TagSet{"method": "publish_out"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsPublishOut)
	Add(&md, p+"message_stats", ms.Redeliver, ts.Copy().Merge(opentsdb.TagSet{"method": "redeliver"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsRedeliver)
	Add(&md, p+"message_stats", ms.Return, ts.Copy().Merge(opentsdb.TagSet{"method": "return"}),
		metadata.Counter, metadata.Message, DescRmqMessageStatsReturn)
	return md
}

func rabbitmqBackingQueueStatus(p string, ts opentsdb.TagSet, bqs rmqBackingQueueStatus) opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	Add(&md, p+"avg_rate", bqs.AvgAckEgressRate, ts.Copy().Merge(opentsdb.TagSet{"method": "ack", "direction": "out"}),
		metadata.Rate, metadata.Message, DescRmqBackingQueueStatusAvgAckEgressRate)
	Add(&md, p+"avg_rate", bqs.AvgAckIngressRate, ts.Copy().Merge(opentsdb.TagSet{"method": "ack", "direction": "in"}),
		metadata.Rate, metadata.Message, DescRmqBackingQueueStatusAvgAckIngressRate)
	Add(&md, p+"avg_rate", bqs.AvgEgressRate, ts.Copy().Merge(opentsdb.TagSet{"method": "noack", "direction": "out"}),
		metadata.Rate, metadata.Message, DescRmqBackingQueueStatusAvgEgressRate)
	Add(&md, p+"avg_rate", bqs.AvgIngressRate, ts.Copy().Merge(opentsdb.TagSet{"method": "noack", "direction": "in"}),
		metadata.Rate, metadata.Message, DescRmqBackingQueueStatusAvgIngressRate)
	Add(&md, p+"len", bqs.Len, ts,
		metadata.Gauge, metadata.Message, DescRmqBackingQueueStatusLen)
	return md
}

func urlUserHost(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.User != nil {
		res := fmt.Sprintf("%s@%s", u.User.Username(), u.Host)
		return res, nil
	}
	res := fmt.Sprintf("%s", u.Host)
	return res, nil
}

type rmqOverview struct {
	ClusterName  string          `json:"cluster_name"`
	MessageStats rmqMessageStats `json:"message_stats"`
	QueueTotals  struct {
		Messages               int `json:"messages"`
		MessagesReady          int `json:"messages_ready"`
		MessagesUnacknowledged int `json:"messages_unacknowledged"`
	} `json:"queue_totals"`
	ObjectTotals struct {
		Consumers   int `json:"consumers"`
		Queues      int `json:"queues"`
		Exchanges   int `json:"exchanges"`
		Connections int `json:"connections"`
		Channels    int `json:"channels"`
	} `json:"object_totals"`
	Node string `json:"node"`
}

type rmqNode struct {
	DiskFree      int64       `json:"disk_free"`
	DiskFreeAlarm bool        `json:"disk_free_alarm"`
	DiskFreeLimit int         `json:"disk_free_limit"`
	FDTotal       int         `json:"fd_total"`
	FDUsed        int         `json:"fd_used"`
	MemAlarm      bool        `json:"mem_alarm"`
	MemLimit      int64       `json:"mem_limit"`
	MemUsed       int         `json:"mem_used"`
	Name          string      `json:"name"`
	Partitions    interface{} `json:"partitions"`
	ProcTotal     int         `json:"proc_total"`
	ProcUsed      int         `json:"proc_used"`
	Processors    int         `json:"processors"`
	RunQueue      int         `json:"run_queue"`
	Running       bool        `json:"running"`
	SocketsTotal  int         `json:"sockets_total"`
	SocketsUsed   int         `json:"sockets_used"`
	Uptime        int         `json:"uptime"`
}

type rmqQueue struct {
	Messages                   int                   `json:"messages"`
	MessagesReady              int                   `json:"messages_ready"`
	MessagesUnacknowledged     int                   `json:"messages_unacknowledged"`
	Consumers                  int                   `json:"consumers"`
	ConsumerUtilisation        interface{}           `json:"consumer_utilisation"`
	Memory                     int                   `json:"memory"`
	SlaveNodes                 interface{}           `json:"slave_nodes"`
	SynchronisedSlaveNodes     interface{}           `json:"synchronised_slave_nodes"`
	DownSlaveNodes             interface{}           `json:"down_slave_nodes"`
	BackingQueueStatus         rmqBackingQueueStatus `json:"backing_queue_status"`
	State                      string                `json:"state"`
	MessagesRAM                int                   `json:"messages_ram"`
	MessagesReadyRAM           int                   `json:"messages_ready_ram"`
	MessagesUnacknowledgedRAM  int                   `json:"messages_unacknowledged_ram"`
	MessagesPersistent         int                   `json:"messages_persistent"`
	MessageBytes               int                   `json:"message_bytes"`
	MessageBytesReady          int                   `json:"message_bytes_ready"`
	MessageBytesUnacknowledged int                   `json:"message_bytes_unacknowledged"`
	MessageBytesRAM            int                   `json:"message_bytes_ram"`
	MessageBytesPersistent     int                   `json:"message_bytes_persistent"`
	Name                       string                `json:"name"`
	Vhost                      string                `json:"vhost"`
	Durable                    bool                  `json:"durable"`
	Node                       string                `json:"node"`
	MessageStats               rmqMessageStats       `json:"message_stats"`
}

type rmqMessageStats struct {
	Ack          int `json:"ack"`
	Confirm      int `json:"confirm"`
	Deliver      int `json:"deliver"`
	DeliverGet   int `json:"deliver_get"`
	DeliverNoAck int `json:"deliver_no_ack"`
	Get          int `json:"get"`
	GetAck       int `json:"get_ack"`
	GetNoAck     int `json:"get_noack"`
	Publish      int `json:"publish"`
	PublishIn    int `json:"publish_in"`
	PublishOut   int `json:"publish_out"`
	Redeliver    int `json:"redeliver"`
	Return       int `json:"return"`
}

type rmqBackingQueueStatus struct {
	Len               int     `json:"len"`
	AvgIngressRate    float64 `json:"avg_ingress_rate"`
	AvgEgressRate     float64 `json:"avg_egress_rate"`
	AvgAckIngressRate float64 `json:"avg_ack_ingress_rate"`
	AvgAckEgressRate  float64 `json:"avg_ack_egress_rate"`
	MirrorSeen        int     `json:"mirror_seen"`
	MirrorSenders     int     `json:"mirror_senders"`
}

const (
	DescRmqBackingQueueStatusAvgAckEgressRate  = "Rate at which unacknowledged message records leave RAM, e.g. because acks arrive or unacked messages are paged out"
	DescRmqBackingQueueStatusAvgAckIngressRate = "Rate at which unacknowledged message records enter RAM, e.g. because messages are delivered requiring acknowledgement"
	DescRmqBackingQueueStatusAvgEgressRate     = "Average egress (outbound) rate, not including messages that straight through to auto-acking consumers."
	DescRmqBackingQueueStatusAvgIngressRate    = "Average ingress (inbound) rate, not including messages that straight through to auto-acking consumers."
	DescRmqBackingQueueStatusLen               = "Total backing queue length."
	DescRmqConsumers                           = "Number of consumers."
	DescRmqConsumerUtilisation                 = "Fraction of the time (between 0.0 and 1.0) that the queue is able to immediately deliver messages to consumers. This can be less than 1.0 if consumers are limited by network congestion or prefetch count."
	DescRmqDiskFreeAlarm                       = "Whether the disk alarm has gone off."
	DescRmqDiskFree                            = "Disk free space in bytes."
	DescRmqDiskFreeLimit                       = "Point at which the disk alarm will go off."
	DescRmqDownSlaveNodes                      = "Count of down nodes having a copy of the queue."
	DescRmqFDTotal                             = "File descriptors available."
	DescRmqFDUsed                              = "Used file descriptors."
	DescRmqIOReadAvgTime                       = "Average wall time (milliseconds) for each disk read operation in the last statistics interval."
	DescRmqIOReadBytes                         = "Total number of bytes read from disk by the persister."
	DescRmqIOReadCount                         = "Total number of read operations by the persister."
	DescRmqIOReopenCount                       = "Total number of times the persister has needed to recycle file handles between queues. In an ideal world this number will be zero; if the number is large, performance might be improved by increasing the number of file handles available to RabbitMQ."
	DescRmqIOSeekAvgTime                       = "Average wall time (milliseconds) for each seek operation in the last statistics interval."
	DescRmqIOSeekCount                         = "Total number of seek operations by the persister."
	DescRmqIOSyncAvgTime                       = "Average wall time (milliseconds) for each sync operation in the last statistics interval."
	DescRmqIOSyncCount                         = "Total number of fsync() operations by the persister."
	DescRmqIOWriteAvgTime                      = "Average wall time (milliseconds) for each write operation in the last statistics interval."
	DescRmqIOWriteBytes                        = "Total number of bytes written to disk by the persister."
	DescRmqIOWriteCount                        = "Total number of write operations by the persister."
	DescRmqMemAlarm                            = ""
	DescRmqMemLimit                            = "Point at which the memory alarm will go off."
	DescRmqMemory                              = "Bytes of memory consumed by the Erlang process associated with the queue, including stack, heap and internal structures."
	DescRmqMemUsed                             = "Memory used in bytes."
	DescRmqMessageBytesPersistent              = "Like messageBytes but counting only those messages which are persistent."
	DescRmqMessageBytesRAM                     = "Like messageBytes but counting only those messages which are in RAM."
	DescRmqMessageBytesReady                   = "Like messageBytes but counting only those messages ready to be delivered to clients."
	DescRmqMessageBytes                        = "Sum of the size of all message bodies in the queue. This does not include the message properties (including headers) or any overhead."
	DescRmqMessageBytesUnacknowledged          = "Like messageBytes but counting only those messages delivered to clients but not yet acknowledged."
	DescRmqMessagesPersistent                  = "Total number of persistent messages in the queue (will always be 0 for transient queues)."
	DescRmqMessagesRAM                         = "Total number of messages which are resident in ram."
	DescRmqMessagesReady                       = "Number of messages ready to be delivered to clients."
	DescRmqMessagesReadyRAM                    = "Number of messages from messagesReady which are resident in ram."
	DescRmqMessages                            = "Sum of ready and unacknowledged messages (queue depth)."
	DescRmqMessageStatsAck                     = "Count of acknowledged messages."
	DescRmqMessageStatsConfirm                 = "Count of messages confirmed."
	DescRmqMessageStatsDeliver                 = "Count of messages delivered in acknowledgement mode to consumers."
	DescRmqMessageStatsDeliverGet              = "Sum of deliver, deliverNoack, get, getNoack."
	DescRmqMessageStatsDeliverNoAck            = "Count of messages delivered in no-acknowledgement mode to consumers."
	DescRmqMessageStatsGet                     = "Count of messages delivered in acknowledgement mode in response to basic.get."
	DescRmqMessageStatsGetNoack                = "Count of messages delivered in no-acknowledgement mode in response to basic.get."
	DescRmqMessageStatsPublish                 = "Count of messages published."
	DescRmqMessageStatsPublishIn               = "Count of messages published \"in\" to an exchange, i.e. not taking account of routing."
	DescRmqMessageStatsPublishOut              = "Count of messages published \"out\" of an exchange, i.e. taking account of routing."
	DescRmqMessageStatsRedeliver               = "Count of subset of messages in deliverGet which had the redelivered flag set."
	DescRmqMessageStatsReturn                  = "Count of messages returned to publisher as unroutable."
	DescRmqMessagesUnacknowledged              = "Number of messages delivered to clients but not yet acknowledged."
	DescRmqMessagesUnacknowledgedRAM           = "Number of messages from messagesUnacknowledged which are resident in ram."
	DescRmqMnesiaDiskTxCount                   = "Number of Mnesia transactions which have been performed that required writes to disk. (e.g. creating a durable queue). Only transactions which originated on this node are included."
	DescRmqMnesiaRAMTxCount                    = "Number of Mnesia transactions which have been performed that did not require writes to disk. (e.g. creating a transient queue). Only transactions which originated on this node are included."
	DescRmqMsgStoreReadCount                   = "Number of messages which have been read from the message store."
	DescRmqMsgStoreWriteCount                  = "Number of messages which have been written to the message store."
	DescRmqObjecttotalsChannels                = "Overall number of channels."
	DescRmqObjectTotalsConnections             = "Overall number of connections."
	DescRmqObjectTotalsConsumers               = "Overall number of consumers."
	DescRmqObjectTotalsExchanges               = "Overall number of exchanges."
	DescRmqObjectTotalsQueues                  = "Overall number of queues."
	DescRmqPartitions                          = "Count of network partitions this node is seeing."
	DescRmqProcessors                          = "Number of cores detected and usable by Erlang."
	DescRmqProcTotal                           = "Maximum number of Erlang processes."
	DescRmqProcUsed                            = "Number of Erlang processes in use."
	DescRmqQueueIndexJournalWriteCount         = "Number of records written to the queue index journal. Each record represents a message being published to a queue, being delivered from a queue, and being acknowledged in a queue."
	DescRmqQueueIndexReadCount                 = "Number of records read from the queue index."
	DescRmqQueueIndexWriteCount                = "Number of records written to the queue index."
	DescRmqQueueTotalsMessages                 = "Overall sum of ready and unacknowledged messages (queue depth)."
	DescRmqQueueTotalsMessagesReady            = "Overall number of messages ready to be delivered to clients."
	DescRmqQueueTotalsMessagesUnacknowledged   = "Overall number of messages delivered to clients but not yet acknowledged."
	DescRmqRunning                             = "Boolean for whether this node is up. Obviously if this is false, most other stats will be missing."
	DescRmqRunQueue                            = "Average number of Erlang processes waiting to run."
	DescRmqSlaveNodes                          = "Count of nodes having a copy of the queue."
	DescRmqSocketsTotal                        = "File descriptors available for use as sockets."
	DescRmqSocketsUsed                         = "File descriptors used as sockets."
	DescRmqState                               = "The state of the queue. Unknown=> -1, Running=> 0, Syncing=> 1, Flow=> 2, Down=> 3"
	DescRmqSynchronisedSlaveNodes              = "Count of nodes having synchronised copy of the queue."
	DescRmqSyncMessages                        = "Count of already synchronised messages on a slave node."
	DescRmqUptime                              = "Node uptime in seconds."
)
