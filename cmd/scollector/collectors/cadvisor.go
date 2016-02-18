package collectors

import (
	"strconv"

	"github.com/google/cadvisor/client"
	"github.com/google/cadvisor/info/v1"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func init() {
	registerInit(startCadvisorCollector)
}

var cadvisorMeta = map[string]MetricMeta{
	"container.cpu.system": {
		RateType: metadata.Counter,
		Unit:     metadata.Second,
		Desc:     "Cumulative system cpu time consumed in seconds.",
	},
	"container.cpu.usage": {
		RateType: metadata.Counter,
		Unit:     metadata.Second,
		Desc:     "Cumulative cpu time consumed per cpu in seconds.",
	},
	"container.cpu.user": {
		RateType: metadata.Counter,
		Unit:     metadata.Second,
		Desc:     "Cumulative user cpu time consumed in seconds.",
	},
	"container.cpu.loadavg": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Smoothed 10s average of number of runnable threads x 1000",
	},
	"container.fs.avalable": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes available for non-root user.",
	},
	"container.fs.limit": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes that can be consumed by the container on this filesystem.",
	},
	"container.fs.usage": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of bytes that is consumed by the container on this filesystem.",
	},
	"container.fs.reads.time": {
		RateType: metadata.Counter,
		Unit:     metadata.MilliSecond,
		Desc:     "Number of milliseconds spent reading",
	},
	"container.fs.reads.merged": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of reads merged",
	},
	"container.fs.reads.sectors": {
		RateType: metadata.Counter,
		Unit:     metadata.Sector,
		Desc:     "Number of sectors read",
	},
	"container.fs.reads": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of reads completed",
	},
	"container.fs.writes.sectors": {
		RateType: metadata.Counter,
		Unit:     metadata.Sector,
		Desc:     "Number of sectors written",
	},
	"container.fs.writes.time": {
		RateType: metadata.Counter,
		Unit:     metadata.MilliSecond,
		Desc:     "Number of milliseconds spent writing",
	},
	"container.fs.writes.merged": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of writes merged",
	},
	"container.fs.writes": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of writes completed",
	},
	"container.fs.io.current": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Number of I/Os currently in progress",
	},
	"container.fs.io.time": {
		RateType: metadata.Counter,
		Unit:     metadata.MilliSecond,
		Desc:     "Number of milliseconds spent doing I/Os",
	},
	"container.fs.io.time.weighted": {
		RateType: metadata.Counter,
		Unit:     metadata.MilliSecond,
		Desc:     "Cumulative weighted I/O time",
	},
	"container.last.seen": {
		RateType: metadata.Gauge,
		Unit:     metadata.None,
	},
	"container.memory.failures": {
		RateType: metadata.Counter,
		Unit:     metadata.Fault,
		Desc:     "Count of memory allocation failure.",
	},
	"container.memory.usage": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Current memory usage.",
	},
	"container.memory.working_set": {
		RateType: metadata.Gauge,
		Unit:     metadata.Bytes,
		Desc:     "Current working set.",
	},
	"container.net.bytes": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
	},
	"container.net.errors": {
		RateType: metadata.Counter,
		Unit:     metadata.Error,
	},
	"container.net.dropped": {
		RateType: metadata.Counter,
		Unit:     metadata.Packet,
	},
	"container.net.packets": {
		RateType: metadata.Counter,
		Unit:     metadata.Packet,
	},
	"container.net.tcp": {
		RateType: metadata.Counter,
		Unit:     metadata.Connection,
		Desc:     "Count of tcp connection states.",
	},
	"container.net.tcp6": {
		RateType: metadata.Counter,
		Unit:     metadata.Connection,
		Desc:     "Count of tcp6 connection states.",
	},
}

func cadvisorAdd(md *opentsdb.MultiDataPoint, name string, value interface{}, ts opentsdb.TagSet) {
	Add(md, name, value, ts, cadvisorMeta[name].RateType, cadvisorMeta[name].Unit, cadvisorMeta[name].Desc)
}

func containerTagSet(ts opentsdb.TagSet, container *v1.ContainerInfo) opentsdb.TagSet {
	var tags opentsdb.TagSet
	if container.Namespace == "docker" {
		tags = opentsdb.TagSet{
			"name":        container.Name,
			"docker_name": container.Aliases[0],
			"docker_id":   container.Aliases[1],
		}
	} else {
		tags = opentsdb.TagSet{
			"name": container.Name,
		}
	}
	for k, v := range ts {
		tags[k] = v
	}
	return tags
}

func statsForContainer(md *opentsdb.MultiDataPoint, container *v1.ContainerInfo) {
	stats := container.Stats[0]
	var ts opentsdb.TagSet
	if container.Spec.HasCpu {
		ts = containerTagSet(ts, container)
		cadvisorAdd(md, "container.cpu.loadavg", stats.Cpu.LoadAverage, ts)
		cadvisorAdd(md, "container.cpu.system", stats.Cpu.Usage.System, ts)
		cadvisorAdd(md, "container.cpu.user", stats.Cpu.Usage.User, ts)

		ts = containerTagSet(opentsdb.TagSet{"cpu": "all"}, container)
		cadvisorAdd(md, "container.cpu.usage", stats.Cpu.Usage.Total, ts)
		for idx := range stats.Cpu.Usage.PerCpu {
			ts = containerTagSet(opentsdb.TagSet{"cpu": strconv.Itoa(idx)}, container)
			cadvisorAdd(md, "container.cpu.usage", stats.Cpu.Usage.PerCpu[idx], ts)
		}
	}

	if container.Spec.HasFilesystem {
		for idx := range stats.Filesystem {
			ts = containerTagSet(opentsdb.TagSet{"device": stats.Filesystem[idx].Device}, container)
			cadvisorAdd(md, "container.fs.avalable", stats.Filesystem[idx].Available, ts)
			cadvisorAdd(md, "container.fs.limit", stats.Filesystem[idx].Limit, ts)
			cadvisorAdd(md, "container.fs.usage", stats.Filesystem[idx].Usage, ts)
			cadvisorAdd(md, "container.fs.reads.time", stats.Filesystem[idx].ReadTime, ts)
			cadvisorAdd(md, "container.fs.reads.merged", stats.Filesystem[idx].ReadsMerged, ts)
			cadvisorAdd(md, "container.fs.reads.sectors", stats.Filesystem[idx].SectorsRead, ts)
			cadvisorAdd(md, "container.fs.reads", stats.Filesystem[idx].ReadsCompleted, ts)
			cadvisorAdd(md, "container.fs.writes.sectors", stats.Filesystem[idx].SectorsWritten, ts)
			cadvisorAdd(md, "container.fs.writes.time", stats.Filesystem[idx].WriteTime, ts)
			cadvisorAdd(md, "container.fs.writes.merged", stats.Filesystem[idx].WritesMerged, ts)
			cadvisorAdd(md, "container.fs.writes", stats.Filesystem[idx].WritesCompleted, ts)
			cadvisorAdd(md, "container.fs.io.current", stats.Filesystem[idx].IoInProgress, ts)
			cadvisorAdd(md, "container.fs.io.time", stats.Filesystem[idx].IoTime, ts)
			cadvisorAdd(md, "container.fs.io.time.weighted", stats.Filesystem[idx].WeightedIoTime, ts)
		}
	}

	if container.Spec.HasMemory {
		cadvisorAdd(md, "container.memory.failures", stats.Memory.ContainerData.Pgfault,
			containerTagSet(opentsdb.TagSet{"scope": "container", "type": "pgfault"}, container))
		cadvisorAdd(md, "container.memory.failures", stats.Memory.ContainerData.Pgmajfault,
			containerTagSet(opentsdb.TagSet{"scope": "container", "type": "pgmajfault"}, container))
		cadvisorAdd(md, "container.memory.failures", stats.Memory.HierarchicalData.Pgfault,
			containerTagSet(opentsdb.TagSet{"scope": "hierarchy", "type": "pgfault"}, container))
		cadvisorAdd(md, "container.memory.failures", stats.Memory.HierarchicalData.Pgmajfault,
			containerTagSet(opentsdb.TagSet{"scope": "hierarchy", "type": "pgmajfault"}, container))
		cadvisorAdd(md, "container.memory.working_set", stats.Memory.WorkingSet, containerTagSet(nil, container))
		cadvisorAdd(md, "container.memory.usage", stats.Memory.Usage, containerTagSet(nil, container))
	}

	if container.Spec.HasNetwork {
		for _, iface := range stats.Network.Interfaces {
			ts = containerTagSet(opentsdb.TagSet{"ifName": iface.Name, "direction": "in"}, container)
			cadvisorAdd(md, "container.net.bytes", iface.RxBytes, ts)
			cadvisorAdd(md, "container.net.errors", iface.RxErrors, ts)
			cadvisorAdd(md, "container.net.dropped", iface.RxDropped, ts)
			cadvisorAdd(md, "container.net.packets", iface.RxPackets, ts)
			ts = containerTagSet(opentsdb.TagSet{"ifName": iface.Name, "direction": "out"}, container)
			cadvisorAdd(md, "container.net.bytes", iface.TxBytes, ts)
			cadvisorAdd(md, "container.net.errors", iface.TxErrors, ts)
			cadvisorAdd(md, "container.net.dropped", iface.TxDropped, ts)
			cadvisorAdd(md, "container.net.packets", iface.TxPackets, ts)
		}
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.Close, containerTagSet(opentsdb.TagSet{"state": "close"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.CloseWait, containerTagSet(opentsdb.TagSet{"state": "closewait"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.Closing, containerTagSet(opentsdb.TagSet{"state": "closing"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.Established, containerTagSet(opentsdb.TagSet{"state": "established"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.FinWait1, containerTagSet(opentsdb.TagSet{"state": "finwait1"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.FinWait2, containerTagSet(opentsdb.TagSet{"state": "finwait2"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.LastAck, containerTagSet(opentsdb.TagSet{"state": "lastack"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.Listen, containerTagSet(opentsdb.TagSet{"state": "listen"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.SynRecv, containerTagSet(opentsdb.TagSet{"state": "synrecv"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.SynSent, containerTagSet(opentsdb.TagSet{"state": "synsent"}, container))
		cadvisorAdd(md, "container.net.tcp", stats.Network.Tcp.TimeWait, containerTagSet(opentsdb.TagSet{"state": "timewait"}, container))

		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.Close, containerTagSet(opentsdb.TagSet{"state": "close"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.CloseWait, containerTagSet(opentsdb.TagSet{"state": "closewait"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.Closing, containerTagSet(opentsdb.TagSet{"state": "closing"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.Established, containerTagSet(opentsdb.TagSet{"state": "established"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.FinWait1, containerTagSet(opentsdb.TagSet{"state": "finwait1"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.FinWait2, containerTagSet(opentsdb.TagSet{"state": "finwait2"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.LastAck, containerTagSet(opentsdb.TagSet{"state": "lastack"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.Listen, containerTagSet(opentsdb.TagSet{"state": "listen"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.SynRecv, containerTagSet(opentsdb.TagSet{"state": "synrecv"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.SynSent, containerTagSet(opentsdb.TagSet{"state": "synsent"}, container))
		cadvisorAdd(md, "container.net.tcp6", stats.Network.Tcp6.TimeWait, containerTagSet(opentsdb.TagSet{"state": "timewait"}, container))
	}
}

func c_cadvisor(c *client.Client) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	containers, err := c.AllDockerContainers(&v1.ContainerInfoRequest{NumStats: 1})
	if err != nil {
		slog.Errorf("Error fetching containers from cadvisor: %v", err)
		return md, err
	}

	for _, container := range containers {
		statsForContainer(&md, &container)
	}

	return md, nil
}

func startCadvisorCollector(c *conf.Conf) {
	for _, config := range c.Cadvisor {
		cClient, err := client.NewClient(config.URL)
		if err != nil {
			slog.Warningf("Could not start collector for URL [%s] due to err: %v", config.URL, err)
		}
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_cadvisor(cClient)
			},
			name: "cadvisor",
		})
	}
}
