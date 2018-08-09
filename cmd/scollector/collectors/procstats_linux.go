package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_procstats_linux})
	collectors = append(collectors, &IntervalCollector{F: c_interrupts_linux, Interval: time.Minute})
	collectors = append(collectors, &IntervalCollector{F: c_vmstat_linux, Interval: time.Minute})
}

var uptimeRE = regexp.MustCompile(`(\S+)\s+(\S+)`)
var meminfoRE = regexp.MustCompile(`(\w+):\s+(\d+)\s+(\w+)`)
var vmstatRE = regexp.MustCompile(`(\w+)\s+(\d+)`)
var statRE = regexp.MustCompile(`(\w+)\s+(.*)`)
var statCPURE = regexp.MustCompile(`cpu(\d+)`)
var cpuspeedRE = regexp.MustCompile(`cpu MHz\s+: ([\d.]+)`)
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
	"steal",
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
		Add(&md, "linux.uptime_total", m[1], nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
		Add(&md, "linux.uptime_now", m[2], nil, metadata.Gauge, metadata.Second, "")
		Add(&md, osSystemUptime, m[1], nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
		return nil
	}); err != nil {
		Error = err
	}
	mem := make(map[string]int64)
	if err := readLine("/proc/meminfo", func(s string) error {
		m := meminfoRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		i, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return err
		}
		mem[m[1]] = i
		Add(&md, "linux.mem."+strings.ToLower(m[1]), m[2], nil, metadata.Gauge, metadata.KBytes, "")
		return nil
	}); err != nil {
		Error = err
	}
	bufferCacheSlab := mem["Buffers"] + mem["Cached"] + mem["Slab"]
	memTotal := mem["MemTotal"]
	memFree := mem["MemFree"]
	// MemAvailable was introduced in the 3.14 kernel and is a more accurate measure of available memory
	// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=34e431b0a
	// We used this metric if it is available
	available, availableIsAvailable := mem["MemAvailable"]
	Add(&md, osMemTotal, memTotal*1024, nil, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
	freeValue := memFree + bufferCacheSlab
	usedValue := memTotal - memFree - bufferCacheSlab
	if availableIsAvailable {
		freeValue = available
		usedValue = memTotal - available
	}
	Add(&md, osMemFree, freeValue*1024, nil, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
	Add(&md, osMemUsed, usedValue*1024, nil, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
	if memTotal != 0 {
		Add(&md, osMemPctFree, (float64(freeValue))/float64(memTotal)*100, nil, metadata.Gauge, metadata.Pct, osMemFreeDesc)
	}

	num_cores := 0
	var t_util float64
	cpu_stat_desc := map[string]string{
		"user":       "Normal processes executing in user mode.",
		"nice":       "Niced processes executing in user mode.",
		"system":     "Processes executing in kernel mode.",
		"idle":       "Twiddling thumbs.",
		"iowait":     "Waiting for I/O to complete.",
		"irq":        "Servicing interrupts.",
		"softirq":    "Servicing soft irqs.",
		"steal":      "Involuntary wait.",
		"guest":      "Running a guest vm.",
		"guest_nice": "Running a niced guest vm.",
	}
	if err := readLine("/proc/stat", func(s string) error {
		m := statRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		if strings.HasPrefix(m[1], "cpu") {
			metric_percpu := ""
			tag_cpu := ""
			cpu_m := statCPURE.FindStringSubmatch(m[1])
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
				Add(&md, "linux.cpu"+metric_percpu, value, tags, metadata.Counter, metadata.CHz, cpu_stat_desc[CPU_FIELDS[i]])
			}
			if metric_percpu == "" {
				if len(fields) < 3 {
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
			Add(&md, "linux.intr", strings.Fields(m[2])[0], nil, metadata.Counter, metadata.Interupt, "")
		} else if m[1] == "ctxt" {
			Add(&md, "linux.ctxt", m[2], nil, metadata.Counter, metadata.ContextSwitch, "")
		} else if m[1] == "processes" {
			Add(&md, "linux.processes", m[2], nil, metadata.Counter, metadata.Process,
				"The number  of processes and threads created, which includes (but  is not limited  to) those  created by  calls to the  fork() and clone() system calls.")
		} else if m[1] == "procs_blocked" {
			Add(&md, "linux.procs_blocked", m[2], nil, metadata.Gauge, metadata.Process, "The  number of  processes currently blocked, waiting for I/O to complete.")
		}
		return nil
	}); err != nil {
		Error = err
	}
	if num_cores != 0 && t_util != 0 {
		Add(&md, osCPU, t_util/float64(num_cores), nil, metadata.Counter, metadata.Pct, "")
	}
	cpuinfo_index := 0
	if err := readLine("/proc/cpuinfo", func(s string) error {
		m := cpuspeedRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		tags := opentsdb.TagSet{"cpu": strconv.Itoa(cpuinfo_index)}
		Add(&md, osCPUClock, m[1], tags, metadata.Gauge, metadata.MHz, osCPUClockDesc)
		Add(&md, "linux.cpu.clock", m[1], tags, metadata.Gauge, metadata.MHz, osCPUClockDesc)
		cpuinfo_index += 1
		return nil
	}); err != nil {
		Error = err
	}
	if err := readLine("/proc/loadavg", func(s string) error {
		m := loadavgRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		Add(&md, "linux.loadavg_1_min", m[1], nil, metadata.Gauge, metadata.Load, "")
		Add(&md, "linux.loadavg_5_min", m[2], nil, metadata.Gauge, metadata.Load, "")
		Add(&md, "linux.loadavg_15_min", m[3], nil, metadata.Gauge, metadata.Load, "")
		Add(&md, "linux.loadavg_runnable", m[4], nil, metadata.Gauge, metadata.Process, "")
		Add(&md, "linux.loadavg_total_threads", m[5], nil, metadata.Gauge, metadata.Process, "")
		return nil
	}); err != nil {
		Error = err
	}
	if err := readLine("/proc/sys/kernel/random/entropy_avail", func(s string) error {
		Add(&md, "linux.entropy_avail", strings.TrimSpace(s), nil, metadata.Gauge, metadata.Entropy, "The remaing amount of entropy available to the system. If it is low or hitting zero processes might be blocked waiting for extropy")
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
			Add(&md, "linux.net.sockets.used", cols[2], nil, metadata.Gauge, metadata.Socket, "")
		case "TCP:":
			if len(cols) < 11 {
				return fmt.Errorf("sockstat: error parsing tcp line")
			}
			Add(&md, "linux.net.sockets.tcp_in_use", cols[2], nil, metadata.Gauge, metadata.Socket, "")
			Add(&md, "linux.net.sockets.tcp_orphaned", cols[4], nil, metadata.Gauge, metadata.Socket, "")
			Add(&md, "linux.net.sockets.tcp_time_wait", cols[6], nil, metadata.Gauge, metadata.Socket, "")
			Add(&md, "linux.net.sockets.tcp_allocated", cols[8], nil, metadata.Gauge, metadata.None, "")
			Add(&md, "linux.net.sockets.tcp_mem", cols[10], nil, metadata.Gauge, metadata.None, "")
		case "UDP:":
			if len(cols) < 5 {
				return fmt.Errorf("sockstat: error parsing udp line")
			}
			Add(&md, "linux.net.sockets.udp_in_use", cols[2], nil, metadata.Gauge, metadata.Socket, "")
			Add(&md, "linux.net.sockets.udp_mem", cols[4], nil, metadata.Gauge, metadata.Page, "")
		case "UDPLITE:":
			if len(cols) < 3 {
				return fmt.Errorf("sockstat: error parsing udplite line")
			}
			Add(&md, "linux.net.sockets.udplite_in_use", cols[2], nil, metadata.Gauge, metadata.Socket, "")
		case "RAW:":
			if len(cols) < 3 {
				return fmt.Errorf("sockstat: error parsing raw line")
			}
			Add(&md, "linux.net.sockets.raw_in_use", cols[2], nil, metadata.Gauge, metadata.Socket, "")
		case "FRAG:":
			if len(cols) < 5 {
				return fmt.Errorf("sockstat: error parsing frag line")
			}
			Add(&md, "linux.net.sockets.frag_in_use", cols[2], nil, metadata.Gauge, metadata.Socket, "")
			Add(&md, "linux.net.sockets.frag_mem", cols[4], nil, metadata.Gauge, metadata.Bytes, "")
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
				i++
				m := "linux.net.stat." + root + "." + strings.TrimPrefix(strings.ToLower(headers[i]), "tcp")
				Add(&md, m, v, nil, metadata.Counter, metadata.None, "")
			}
		}
		ln += 1
		return nil
	}); err != nil {
		Error = err
	}
	ln = 0
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
				var stype metadata.RateType = metadata.Counter
				stat := strings.ToLower(headers[i])
				if strings.HasPrefix(stat, "rto") {
					stype = metadata.Gauge
				}
				Add(&md, "linux.net.stat."+proto+"."+stat, v, nil, stype, metadata.None, "")
			}
		}
		return nil
	}); err != nil {
		Error = err
	}
	if err := readLine("/proc/sys/fs/file-nr", func(s string) error {
		f := strings.Fields(s)
		if len(f) != 3 {
			return fmt.Errorf("unexpected number of fields")
		}
		v, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			return err
		}
		Add(&md, "linux.fs.open", v, nil, metadata.Gauge, metadata.Count, "The number of files presently open.")
		return nil
	}); err != nil {
		Error = err
	}
	return md, Error
}

func c_interrupts_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	irq_type_desc := map[string]string{
		"NMI": "Non-maskable interrupts.",
		"LOC": "Local timer interrupts.",
		"SPU": "Spurious interrupts.",
		"PMI": "Performance monitoring interrupts.",
		"IWI": "IRQ work interrupts.",
		"RES": "Rescheduling interrupts.",
		"CAL": "Funcation call interupts.",
		"TLB": "TLB (translation lookaside buffer) shootdowns.",
		"TRM": "Thermal event interrupts.",
		"THR": "Threshold APIC interrupts.",
		"MCE": "Machine check exceptions.",
		"MCP": "Machine Check polls.",
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
			if i >= num_cpus || !IsDigit(val) {
				// All values read, remaining cols contain textual description.
				break
			}
			Add(&md, "linux.interrupts", val, opentsdb.TagSet{"type": irq_type, "cpu": strconv.Itoa(i)}, metadata.Counter, metadata.Interupt, irq_type_desc[irq_type])
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return md, nil
}

func c_vmstat_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	if err := readLine("/proc/vmstat", func(s string) error {
		m := vmstatRE.FindStringSubmatch(s)
		if m == nil {
			return nil
		}
		switch m[1] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout", "pgfault", "pgmajfault":
			mio := inoutRE.FindStringSubmatch(m[1])
			if mio != nil {
				Add(&md, "linux.mem."+mio[1], m[2], opentsdb.TagSet{"direction": mio[2]}, metadata.Counter, metadata.Page, "")
			} else {
				Add(&md, "linux.mem."+m[1], m[2], nil, metadata.Counter, metadata.Page, "")
			}
		default:
			Add(&md, "linux.mem."+m[1], m[2], nil, metadata.Counter, metadata.None, "")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return md, nil
}
