package collectors

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/StackExchange/tcollector/opentsdb"
)

func init() {
	collectors = append(collectors, Collector{F: c_procstats_linux})
}

var uptimeRE = regexp.MustCompile(`(\S+)\s+(\S+)`)
var meminfoRE = regexp.MustCompile(`(\w+):\s+(\d+)\s+(\w+)`)
var vmstatRE = regexp.MustCompile(`(\w+)\s+(\d+)`)
var statRE = regexp.MustCompile(`(\w+)\s+(.*)`)
var statCpuRE = regexp.MustCompile(`cpu(\d+)`)
var loadavgRE = regexp.MustCompile(`(\S+)\s+(\S+)\s+(\S+)\s+(\d+)/(\d+)\s+`)

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
	readProc("/proc/uptime", func(s string) {
		m := uptimeRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "proc.uptime.total", m[1], nil)
		Add(&md, "proc.uptime.now", m[2], nil)
	})
	readProc("/proc/meminfo", func(s string) {
		m := meminfoRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "proc.meminfo."+strings.ToLower(m[1]), m[2], nil)
	})
	readProc("/proc/vmstat", func(s string) {
		m := vmstatRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		switch m[1] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout", "pgfault", "pgmajfault":
			Add(&md, "proc.vmstat."+m[1], m[2], nil)
		}
	})
	readProc("/proc/stat", func(s string) {
		m := statRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		if strings.HasPrefix(m[1], "cpu") {
			metric_percpu := ""
			tag_cpu := ""
			cpu_m := statCpuRE.FindStringSubmatch(m[1])
			if cpu_m != nil {
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
				Add(&md, "proc.stat.cpu"+metric_percpu, value, tags)
			}
		} else if m[1] == "intr" {
			Add(&md, "proc.stat.intr", strings.Fields(m[2])[0], nil)
		} else if m[1] == "ctxt" {
			Add(&md, "proc.stat.ctxt", m[2], nil)
		} else if m[1] == "processes" {
			Add(&md, "proc.stat.processes", m[2], nil)
		} else if m[1] == "procs_blocked" {
			Add(&md, "proc.stat.procs_blocked", m[2], nil)
		}
	})
	readProc("/proc/loadavg", func(s string) {
		m := loadavgRE.FindStringSubmatch(s)
		if m == nil {
			return
		}
		Add(&md, "proc.loadavg.1min", m[1], nil)
		Add(&md, "proc.loadavg.5min", m[2], nil)
		Add(&md, "proc.loadavg.15min", m[3], nil)
		Add(&md, "proc.loadavg.runnable", m[4], nil)
		Add(&md, "proc.loadavg.total_threads", m[5], nil)
	})
	readProc("/proc/sys/kernel/random/entropy_avail", func(s string) {
		Add(&md, "proc.kernel.entropy_avail", strings.TrimSpace(s), nil)
	})
	num_cpus := 0
	readProc("/proc/interrupts", func(s string) {
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
				l.Println("interrupts: unexpected value", val)
				break
			}
			Add(&md, "proc.interrupts", val, opentsdb.TagSet{"type": irq_type, "cpu": strconv.Itoa(i)})
		}
	})
	return md
}
