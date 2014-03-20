package collectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
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

func c_procstats_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	readLine("/proc/uptime", func(s string) {
		m := uptimeRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "linux.uptime_total", m[1], nil)
		Add(&md, "linux.uptime_now", m[2], nil)
	})
	mem := make(map[string]float64)
	readLine("/proc/meminfo", func(s string) {
		m := meminfoRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		i, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			slog.Errorln(err)
		}
		mem[m[1]] = i
		Add(&md, "linux.mem."+strings.ToLower(m[1]), m[2], nil)
	})
	Add(&md, osMemTotal, int(mem["MemTotal"])*1024, nil)
	Add(&md, osMemFree, int(mem["MemFree"])*1024, nil)
	Add(&md, osMemUsed, (int(mem["MemTotal"])-(int(mem["MemFree"])+int(mem["Buffers"])+int(mem["Cached"])))*1024, nil)
	if mem["MemTotal"] != 0 {
		Add(&md, osMemPctFree, (mem["MemFree"]+mem["Buffers"]+mem["Cached"])/mem["MemTotal"]*100, nil)
	}

	readLine("/proc/vmstat", func(s string) {
		m := vmstatRE.FindStringSubmatch(s)
		if m == nil {
			return
		}

		switch m[1] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout", "pgfault", "pgmajfault":
			mio := inoutRE.FindStringSubmatch(m[1])
			if mio != nil {
				Add(&md, "linux.mem."+mio[1], m[2], opentsdb.TagSet{"direction": mio[2]})
			} else {
				Add(&md, "linux.mem."+m[1], m[2], nil)
			}
		}
	})
	num_cores := 0
	readLine("/proc/stat", func(s string) {
		m := statRE.FindStringSubmatch(s)
		if m == nil {
			return
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
				Add(&md, "linux.cpu"+metric_percpu, value, tags)
			}
		} else if m[1] == "intr" {
			Add(&md, "linux.intr", strings.Fields(m[2])[0], nil)
		} else if m[1] == "ctxt" {
			Add(&md, "linux.ctxt", m[2], nil)
		} else if m[1] == "processes" {
			Add(&md, "linux.processes", m[2], nil)
		} else if m[1] == "procs_blocked" {
			Add(&md, "linux.procs_blocked", m[2], nil)
		}
	})
	readLine("/proc/loadavg", func(s string) {
		m := loadavgRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "linux.loadavg_1_min", m[1], nil)
		Add(&md, "linux.loadavg_5_min", m[2], nil)
		Add(&md, "linux.loadavg_15_min", m[3], nil)
		Add(&md, "linux.loadavg_runnable", m[4], nil)
		Add(&md, "linux.loadavg_total_threads", m[5], nil)
	})
	readLine("/proc/sys/kernel/random/entropy_avail", func(s string) {
		Add(&md, "linux.entropy_avail", strings.TrimSpace(s), nil)
	})
	num_cpus := 0
	readLine("/proc/interrupts", func(s string) {
		cols := strings.Fields(s)
		if num_cpus == 0 {
			num_cpus = len(cols)
			return
		} else if len(cols) < 2 {
			return
		}
		irq_type := strings.TrimRight(cols[0], ":")
		if !IsAlNum(irq_type) {
			return
		}
		if IsDigit(irq_type) {
			if cols[len(cols)-2] == "PCI-MSI-edge" && strings.Contains(cols[len(cols)-1], "eth") {
				irq_type = cols[len(cols)-1]
			} else {
				// Interrupt type is just a number, ignore.
				return
			}
		}
		for i, val := range cols[1:] {
			if i >= num_cpus {
				// All values read, remaining cols contain textual description.
				break
			}
			if !IsDigit(val) {
				// Something is weird, there should only be digit values.
				slog.Infoln("interrupts: unexpected value", val)
				break
			}
			Add(&md, "linux.interrupts", val, opentsdb.TagSet{"type": irq_type, "cpu": strconv.Itoa(i)})
		}
	})
	readLine("/proc/uptime", func(s string) {
		cols := strings.Fields(s)
		if len(cols) < 2 {
			return
		}
		total_time, err := strconv.ParseFloat(cols[0], 64)
		idle_time, err := strconv.ParseFloat(cols[1], 64)
		if err != nil {
			return
		}
		if num_cores != 0 {
			Add(&md, osCPU, (total_time-(idle_time/float64(num_cores)))*100, nil)
		}
	})
	readLine("/proc/net/sockstat", func(s string) {
		cols := strings.Fields(s)
		switch cols[0] {
		case "sockets:":
			if len(cols) < 3 {
				slog.Infoln("sockstat: error parsing sockets line")
				break
			}
			Add(&md, "linux.net.sockets.used", cols[2], nil)
		case "TCP:":
			if len(cols) < 11 {
				slog.Infoln("sockstat: error parsing tcp line")
				break
			}
			Add(&md, "linux.net.sockets.tcp_in_use", cols[2], nil)
			Add(&md, "linux.net.sockets.tcp_orphaned", cols[4], nil)
			Add(&md, "linux.net.sockets.tcp_time_wait", cols[6], nil)
			Add(&md, "linux.net.sockets.tcp_allocated", cols[8], nil)
			Add(&md, "linux.net.sockets.tcp_mem", cols[10], nil)
		case "UDP:":
			if len(cols) < 5 {
				slog.Infoln("sockstat: error parsing udp line")
				break
			}
			Add(&md, "linux.net.sockets.udp_in_use", cols[2], nil)
			Add(&md, "linux.net.sockets.udp_mem", cols[4], nil)
		case "UDPLITE:":
			if len(cols) < 3 {
				slog.Infoln("sockstat: error parsing udplite line")
				break
			}
			Add(&md, "linux.net.sockets.udplite_in_use", cols[2], nil)
		case "RAW:":
			if len(cols) < 3 {
				slog.Infoln("sockstat: error parsing raw line")
				break
			}
			Add(&md, "linux.net.sockets.raw_in_use", cols[2], nil)
		case "FRAG:":
			if len(cols) < 5 {
				slog.Infoln("sockstat: error parsing frag line")
				break
			}
			Add(&md, "linux.net.sockets.frag_in_use", cols[2], nil)
			Add(&md, "linux.net.sockets.frag_mem", cols[4], nil)
		}
	})
	ln := 0
	var headers []string
	readLine("/proc/net/netstat", func(s string) {
		cols := strings.Fields(s)
		if ln%2 == 0 {
			headers = cols
		} else {
			if len(cols) < 1 || len(cols) != len(headers) {
				slog.Warningln("netstat: parsing failed")
				return
			}
			root := strings.ToLower(strings.TrimSuffix(headers[0], "Ext:"))
			for i, v := range cols[1:] {
				i += 1
				m := "linux.net.stat." + root + "." + strings.TrimPrefix(strings.ToLower(headers[i]), "tcp")
				Add(&md, m, v, nil)
			}
		}
		ln += 1
	})
	return md
}
