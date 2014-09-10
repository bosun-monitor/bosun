package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_procstats_linux})
}

var uptimeRE = regexp.MustCompile(`(\S+)\s+(\S+)`)
var meminfoRE = regexp.MustCompile(`(\w+):\s+(\d+)\s+(\w+)`)
var vmstatRE = regexp.MustCompile(`(\w+)\s+(\d+)`)
var statRE = regexp.MustCompile(`(\w+)\s+(.*)`)
var statCpuRE = regexp.MustCompile(`cpu(\d+)`)
var loadavgRE = regexp.MustCompile(`(\S+)\s+(\S+)\s+(\S+)\s+(\d+)/(\d+)\s+`)
var inoutRE = regexp.MustCompile(`(.*)(in|out)`)

var CPU_FIELDS = []string{
	"user",
	"nice",
	"system",
	"idle",
	"iowait",
	"irq",
	"softirq",
	"guest",
	"guest_nice",
}

func c_procstats_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var Error error
	if err := readLine("/proc/uptime", func(s string) error {
		m := uptimeRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		Add(&md, "linux.uptime_total", m[1], nil, metadata.Unknown, metadata.None, "")
		Add(&md, "linux.uptime_now", m[2], nil, metadata.Unknown, metadata.None, "")
		return nil
	}); err != nil {
		Error = err
	}
	mem := make(map[string]float64)
	if err := readLine("/proc/meminfo", func(s string) error {
		m := meminfoRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		i, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return err
		}
		mem[m[1]] = i
		Add(&md, "linux.mem."+strings.ToLower(m[1]), m[2], nil, metadata.Unknown, metadata.None, "")
		return nil
	}); err != nil {
		Error = err
	}
	Add(&md, osMemTotal, int(mem["MemTotal"])*1024, nil, metadata.Unknown, metadata.None, "")
	Add(&md, osMemFree, int(mem["MemFree"])*1024, nil, metadata.Unknown, metadata.None, "")
	Add(&md, osMemUsed, (int(mem["MemTotal"])-(int(mem["MemFree"])+int(mem["Buffers"])+int(mem["Cached"])))*1024, nil, metadata.Unknown, metadata.None, "")
	if mem["MemTotal"] != 0 {
		Add(&md, osMemPctFree, (mem["MemFree"]+mem["Buffers"]+mem["Cached"])/mem["MemTotal"]*100, nil, metadata.Unknown, metadata.None, "")
	}

	if err := readLine("/proc/vmstat", func(s string) error {
		m := vmstatRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}

		switch m[1] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout", "pgfault", "pgmajfault":
			mio := inoutRE.FindStringSubmatch(m[1])
			if mio != nil {
				Add(&md, "linux.mem."+mio[1], m[2], opentsdb.TagSet{"direction": mio[2]}, metadata.Unknown, metadata.None, "")
			} else {
				Add(&md, "linux.mem."+m[1], m[2], nil, metadata.Unknown, metadata.None, "")
			}
		}
		return nil
	}); err != nil {
		Error = err
	}
	num_cores := 0
	var t_util float64
	if err := readLine("/proc/stat", func(s string) error {
		m := statRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		if strings.HasPrefix(m[1], "cpu") {
			metric_percpu := ""
			tag_cpu := ""
			cpu_m := statCpuRE.FindStringSubmatch(m[1])
			if cpu_m != nil {
				num_cores += 1
				metric_percpu = ".percpu"
				tag_cpu = cpu_m[1]
			}
			fields := strings.Fields(m[2])
			for i, value := range fields {
				if i >= len(CPU_FIELDS) {
					break
				}
				tags := opentsdb.TagSet{
					"type": CPU_FIELDS[i],
				}
				if tag_cpu != "" {
					tags["cpu"] = tag_cpu
				}
				Add(&md, "linux.cpu"+metric_percpu, value, tags, metadata.Unknown, metadata.None, "")
			}
			if metric_percpu == "" {
				if len(fields) != len(CPU_FIELDS) {
					return nil
				}
				user, err := strconv.ParseFloat(fields[0], 64)
				if err != nil {
					return nil
				}
				nice, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					return nil
				}
				system, err := strconv.ParseFloat(fields[2], 64)
				if err != nil {
					return nil
				}
				t_util = user + nice + system
			}
		} else if m[1] == "intr" {
			Add(&md, "linux.intr", strings.Fields(m[2])[0], nil, metadata.Unknown, metadata.None, "")
		} else if m[1] == "ctxt" {
			Add(&md, "linux.ctxt", m[2], nil, metadata.Unknown, metadata.None, "")
		} else if m[1] == "processes" {
			Add(&md, "linux.processes", m[2], nil, metadata.Unknown, metadata.None, "")
		} else if m[1] == "procs_blocked" {
			Add(&md, "linux.procs_blocked", m[2], nil, metadata.Unknown, metadata.None, "")
		}
		return nil
	}); err != nil {
		Error = err
	}
	if num_cores != 0 && t_util != 0 {
		Add(&md, osCPU, t_util/float64(num_cores), nil, metadata.Unknown, metadata.None, "")
	}
	if err := readLine("/proc/loadavg", func(s string) error {
		m := loadavgRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		Add(&md, "linux.loadavg_1_min", m[1], nil, metadata.Unknown, metadata.None, "")
		Add(&md, "linux.loadavg_5_min", m[2], nil, metadata.Unknown, metadata.None, "")
		Add(&md, "linux.loadavg_15_min", m[3], nil, metadata.Unknown, metadata.None, "")
		Add(&md, "linux.loadavg_runnable", m[4], nil, metadata.Unknown, metadata.None, "")
		Add(&md, "linux.loadavg_total_threads", m[5], nil, metadata.Unknown, metadata.None, "")
		return nil
	}); err != nil {
		Error = err
	}
	if err := readLine("/proc/sys/kernel/random/entropy_avail", func(s string) error {
		Add(&md, "linux.entropy_avail", strings.TrimSpace(s), nil, metadata.Unknown, metadata.None, "")
		return nil
	}); err != nil {
		Error = err
	}
	num_cpus := 0
	if err := readLine("/proc/interrupts", func(s string) error {
		cols := strings.Fields(s)
		if num_cpus == 0 {
			num_cpus = len(cols)
			return nil
		} else if len(cols) < 2 {
			return nil
		}
		irq_type := strings.TrimRight(cols[0], ":")
		if !IsAlNum(irq_type) {
			return nil
		}
		if IsDigit(irq_type) {
			if cols[len(cols)-2] == "PCI-MSI-edge" && strings.Contains(cols[len(cols)-1], "eth") {
				irq_type = cols[len(cols)-1]
			} else {
				// Interrupt type is just a number, ignore.
				return nil
			}
		}
		for i, val := range cols[1:] {
			if i >= num_cpus {
				// All values read, remaining cols contain textual description.
				break
			}
			if !IsDigit(val) {
				// Something is weird, there should only be digit values.
				return fmt.Errorf("interrupts: unexpected value: %v", val)
				break
			}
			Add(&md, "linux.interrupts", val, opentsdb.TagSet{"type": irq_type, "cpu": strconv.Itoa(i)}, metadata.Unknown, metadata.None, "")
		}
		return nil
	}); err != nil {
		Error = err
	}
	if err := readLine("/proc/net/sockstat", func(s string) error {
		cols := strings.Fields(s)
		switch cols[0] {
		case "sockets:":
			if len(cols) < 3 {
				return fmt.Errorf("sockstat: error parsing sockets line")
			}
			Add(&md, "linux.net.sockets.used", cols[2], nil, metadata.Unknown, metadata.None, "")
		case "TCP:":
			if len(cols) < 11 {
				return fmt.Errorf("sockstat: error parsing tcp line")
			}
			Add(&md, "linux.net.sockets.tcp_in_use", cols[2], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.tcp_orphaned", cols[4], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.tcp_time_wait", cols[6], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.tcp_allocated", cols[8], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.tcp_mem", cols[10], nil, metadata.Unknown, metadata.None, "")
		case "UDP:":
			if len(cols) < 5 {
				return fmt.Errorf("sockstat: error parsing udp line")
			}
			Add(&md, "linux.net.sockets.udp_in_use", cols[2], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.udp_mem", cols[4], nil, metadata.Unknown, metadata.None, "")
		case "UDPLITE:":
			if len(cols) < 3 {
				return fmt.Errorf("sockstat: error parsing udplite line")
			}
			Add(&md, "linux.net.sockets.udplite_in_use", cols[2], nil, metadata.Unknown, metadata.None, "")
		case "RAW:":
			if len(cols) < 3 {
				return fmt.Errorf("sockstat: error parsing raw line")
			}
			Add(&md, "linux.net.sockets.raw_in_use", cols[2], nil, metadata.Unknown, metadata.None, "")
		case "FRAG:":
			if len(cols) < 5 {
				return fmt.Errorf("sockstat: error parsing frag line")
			}
			Add(&md, "linux.net.sockets.frag_in_use", cols[2], nil, metadata.Unknown, metadata.None, "")
			Add(&md, "linux.net.sockets.frag_mem", cols[4], nil, metadata.Unknown, metadata.None, "")
		}
		return nil
	}); err != nil {
		Error = err
	}
	ln := 0
	var headers []string
	if err := readLine("/proc/net/netstat", func(s string) error {
		cols := strings.Fields(s)
		if ln%2 == 0 {
			headers = cols
		} else {
			if len(cols) < 1 || len(cols) != len(headers) {
				return fmt.Errorf("netstat: parsing failed")
			}
			root := strings.ToLower(strings.TrimSuffix(headers[0], "Ext:"))
			for i, v := range cols[1:] {
				i += 1
				m := "linux.net.stat." + root + "." + strings.TrimPrefix(strings.ToLower(headers[i]), "tcp")
				Add(&md, m, v, nil, metadata.Unknown, metadata.None, "")
			}
		}
		ln += 1
		return nil
	}); err != nil {
		Error = err
	}
	ln = 0
	metric := "linux.net.stat."
	if err := readLine("/proc/net/snmp", func(s string) error {
		ln++
		if ln%2 != 0 {
			f := strings.Fields(s)
			if len(f) < 2 {
				return fmt.Errorf("Failed to parse header line")
			}
			headers = f
		} else {
			values := strings.Fields(s)
			if len(values) != len(headers) {
				return fmt.Errorf("Mismatched header and value length")
			}
			proto := strings.ToLower(strings.TrimSuffix(values[0], ":"))
			for i, v := range values {
				if i == 0 {
					continue
				}
				stat := strings.ToLower(headers[i])
				if strings.HasPrefix(stat, "rto") {
					Add(&md, metric+proto+"."+stat, v, nil, metadata.Gauge, metadata.None, "")
					continue
				}
				Add(&md, metric+proto+"."+stat, v, nil, metadata.Counter, metadata.None, "")
			}
		}
		return nil
	}); err != nil {
		Error = err
	}
	return md, Error
}
