package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

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
	"container.cpu": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Cumulative cpu time consumed in user/system in nanoseconds.",
	},
	"container.cpu.usage": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Cumulative cpu time consumed in nanoseconds.",
	},
	"container.cpu.usage.percpu": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Cumulative cpu time consumed per cpu in nanoseconds.",
	},
	"container.cpu.loadavg": {
		RateType: metadata.Gauge,
		Unit:     metadata.Second,
		Desc:     "Smoothed 10s average of number of runnable threads x 1000",
	},
	"container.blkio.io_service_bytes.async": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes transferred to/from the disk by the cgroup asynchronously",
	},
	"container.blkio.io_service_bytes.read": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes read from the disk by the cgroup",
	},
	"container.blkio.io_service_bytes.sync": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes transferred to/from the disk by the cgroup synchronously",
	},
	"container.blkio.io_service_bytes.write": {
		RateType: metadata.Counter,
		Unit:     metadata.Bytes,
		Desc:     "Number of bytes written to the disk by the cgroup",
	},
	"container.blkio.io_serviced.async": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of async IOs issued to the disk by the cgroup",
	},
	"container.blkio.io_serviced.read": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of read issued to the disk by the group",
	},
	"container.blkio.io_serviced.sync": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of sync IOs issued to the disk by the cgroup",
	},
	"container.blkio.io_serviced.write": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of write issued to the disk by the group",
	},
	"container.blkio.io_queued.async": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of async requests queued up at any given instant for this cgroup",
	},
	"container.blkio.io_queued.read": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of read requests queued up at any given instant for this cgroup",
	},
	"container.blkio.io_queued.sync": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of sync requests queued up at any given instant for this cgroup",
	},
	"container.blkio.io_queued.write": {
		RateType: metadata.Gauge,
		Unit:     metadata.Operation,
		Desc:     "Total number of write requests queued up at any given instant for this cgroup",
	},
	"container.blkio.sectors.count": {
		RateType: metadata.Counter,
		Unit:     metadata.Sector,
		Desc:     "Number of sectors transferred to/from disk by the group",
	},
	"container.blkio.io_service_time.async": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time between async request dispatch and request completion for the IOs done by this cgroup",
	},
	"container.blkio.io_service_time.read": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time between read request dispatch and request completion for the IOs done by this cgroup",
	},
	"container.blkio.io_service_time.sync": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time between sync request dispatch and request completion for the IOs done by this cgroup",
	},
	"container.blkio.io_service_time.write": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time between write request dispatch and request completion for the IOs done by this cgroup",
	},
	"container.blkio.io_wait_time.async": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time the async IOs for this cgroup spent waiting in the scheduler queues for service",
	},
	"container.blkio.io_wait_time.read": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time the read request for this cgroup spent waiting in the scheduler queues for service",
	},
	"container.blkio.io_wait_time.sync": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time the sync IOs for this cgroup spent waiting in the scheduler queues for service",
	},
	"container.blkio.io_wait_time.write": {
		RateType: metadata.Counter,
		Unit:     metadata.Nanosecond,
		Desc:     "Total amount of time the write request for this cgroup spent waiting in the scheduler queues for service",
	},
	"container.blkio.io_merged.async": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of async requests merged into requests belonging to this cgroup.",
	},
	"container.blkio.io_merged.read": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of read requests merged into requests belonging to this cgroup.",
	},
	"container.blkio.io_merged.sync": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of sync requests merged into requests belonging to this cgroup.",
	},
	"container.blkio.io_merged.write": {
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Total number of write requests merged into requests belonging to this cgroup.",
	},
	"container.blkio.io_time.count": {
		RateType: metadata.Counter,
		Unit:     metadata.MilliSecond,
		Desc:     "Disk time allocated to cgroup per device",
	},
	"container.fs.available": {
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

var blkioStatsWhitelist = []string{"Async", "Sync", "Read", "Write", "Count"}

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

func inBlkioWhitelist(name string) bool {
	valid := false
	for _, n := range blkioStatsWhitelist {
		if n == name {
			valid = true
			break
		}
	}
	return valid
}

func addBlkioStat(md *opentsdb.MultiDataPoint, name string, diskStats v1.PerDiskStats, container *v1.ContainerInfo) {
	device := blockDeviceLookup(diskStats.Major, diskStats.Minor)
	for label, val := range diskStats.Stats {
		if inBlkioWhitelist(label) {
			cadvisorAdd(md, name+strings.ToLower(label), val, containerTagSet(opentsdb.TagSet{"dev": device}, container))
		}
	}
}

func blockDeviceLookup(major, minor uint64) string {
	blockDevideLoopkupFallback := func(major, minor uint64) string {
		slog.Errorf("Unable to perform lookup under /sys/dev/ for block device major(%d) minor(%d)", major, minor)
		return fmt.Sprintf("major%d_minor%d", major, minor)
	}

	path := fmt.Sprintf("/sys/dev/block/%d:%d/uevent", major, minor)
	file, err := os.Open(path)
	if err != nil {
		return blockDevideLoopkupFallback(major, minor)
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return blockDevideLoopkupFallback(major, minor)
	}

	startIdx := strings.Index(string(content), "DEVNAME=")
	if startIdx == -1 {
		return blockDevideLoopkupFallback(major, minor)
	}

	// Start after the =
	startIdx += 7

	endIdx := strings.Index(string(content[startIdx:]), "\n")
	if endIdx == -1 {
		return blockDevideLoopkupFallback(major, minor)
	}

	return string(content[startIdx : startIdx+endIdx])
}

func statsForContainer(md *opentsdb.MultiDataPoint, container *v1.ContainerInfo, perCpuUsage bool) {
	stats := container.Stats[0]
	var ts opentsdb.TagSet
	if container.Spec.HasCpu {
		cadvisorAdd(md, "container.cpu", stats.Cpu.Usage.System, containerTagSet(opentsdb.TagSet{"type": "system"}, container))
		cadvisorAdd(md, "container.cpu", stats.Cpu.Usage.User, containerTagSet(opentsdb.TagSet{"type": "user"}, container))

		ts = containerTagSet(ts, container)
		cadvisorAdd(md, "container.cpu.loadavg", stats.Cpu.LoadAverage, ts)
		cadvisorAdd(md, "container.cpu.usage", stats.Cpu.Usage.Total, ts)

		if perCpuUsage {
			for idx := range stats.Cpu.Usage.PerCpu {
				ts = containerTagSet(opentsdb.TagSet{"cpu": strconv.Itoa(idx)}, container)
				cadvisorAdd(md, "container.cpu.usage.percpu", stats.Cpu.Usage.PerCpu[idx], ts)
			}
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

	if container.Spec.HasDiskIo {
		for _, d := range stats.DiskIo.IoServiceBytes {
			addBlkioStat(md, "container.blkio.io_service_bytes.", d, container)
		}

		for _, d := range stats.DiskIo.IoServiced {
			addBlkioStat(md, "container.blkio.io_serviced.", d, container)
		}

		for _, d := range stats.DiskIo.IoQueued {
			addBlkioStat(md, "container.blkio.io_service_queued.", d, container)
		}

		for _, d := range stats.DiskIo.Sectors {
			addBlkioStat(md, "container.blkio.sectors.", d, container)
		}

		for _, d := range stats.DiskIo.IoServiceTime {
			addBlkioStat(md, "container.blkio.io_service_time.", d, container)
		}

		for _, d := range stats.DiskIo.IoWaitTime {
			addBlkioStat(md, "container.blkio.io_wait_time.", d, container)
		}

		for _, d := range stats.DiskIo.IoMerged {
			addBlkioStat(md, "container.blkio.io_merged.", d, container)
		}

		for _, d := range stats.DiskIo.IoTime {
			addBlkioStat(md, "container.blkio.io_time.", d, container)
		}
	}
}

func c_cadvisor(c *client.Client, perCpuUsage bool) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	containers, err := c.AllDockerContainers(&v1.ContainerInfoRequest{NumStats: 1})
	if err != nil {
		slog.Errorf("Error fetching containers from cadvisor: %v", err)
		return md, err
	}

	for _, container := range containers {
		statsForContainer(&md, &container, perCpuUsage)
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
				return c_cadvisor(cClient, config.PerCpuUsage)
			},
			name: "cadvisor",
		})
	}
}
