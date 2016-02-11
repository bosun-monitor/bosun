package collectors

import (
	"fmt"
	"strconv"
	"strings"

	"bosun.org/_third_party/golang.org/x/sys/unix"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_procstats_linux})
}

var cpu_fields = []string{
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

var cpu_stat_desc = []string{
	"Normal processes executing in user mode.",
	"Niced processes executing in user mode.",
	"Processes executing in kernel mode.",
	"Twiddling thumbs.",
	"Waiting for I/O to complete.",
	"Servicing interrupts.",
	"Servicing soft irqs.",
	"Involuntary wait.",
	"Running a guest vm.",
	"Running a niced guest vm.",
}

func c_procstats_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var Error error
	var sys unix.Sysinfo_t

	unix.Sysinfo(&sys)

	Add(&md, "linux.uptime_total", sys.Uptime, nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
	Add(&md, osSystemUptime, sys.Uptime, nil, metadata.Gauge, metadata.Second, osSystemUptimeDesc)
	if err := readLine("/proc/meminfo", func(s string) error {
		s = strings.TrimSuffix(s, " kB")
		m := strings.Split(s, ":")
		if m == nil {
			return nil
		}
		m[1] = strings.TrimSpace(m[1])
		Add(&md, "linux.mem."+strings.ToLower(m[0]), m[1], nil, metadata.Gauge, metadata.KBytes, "")
		return nil
	}); err != nil {
		Error = err
	}
	Add(&md, osMemTotal, sys.Totalram*uint64(sys.Unit), nil, metadata.Gauge, metadata.Bytes, osMemTotalDesc)
	Add(&md, osMemFree, sys.Freeram*uint64(sys.Unit), nil, metadata.Gauge, metadata.Bytes, osMemFreeDesc)
	Add(&md, osMemUsed, (sys.Totalram-sys.Freeram)*uint64(sys.Unit), nil, metadata.Gauge, metadata.Bytes, osMemUsedDesc)
	Add(&md, "linux.loadavg.1_min", sys.Loads[0], nil, metadata.Gauge, metadata.Load, "")
	Add(&md, "linux.loadavg.5_min", sys.Loads[1], nil, metadata.Gauge, metadata.Load, "")
	Add(&md, "linux.loadavg.15_min", sys.Loads[2], nil, metadata.Gauge, metadata.Load, "")
	Add(&md, "linux.loadavg.total_threads", sys.Procs, nil, metadata.Gauge, metadata.Process, "")
	if sys.Totalram != 0 {
		Add(&md, osMemPctFree, (sys.Freeram)/(sys.Totalram)*100, nil, metadata.Gauge, metadata.Pct, osMemFreeDesc)
	}
	if err := readLine("/proc/vmstat", func(s string) error {
		m := strings.Split(s, " ")
		if m == nil {
			return nil
		}
		switch m[0] {
		case "pgpgin", "pgpgout", "pswpin", "pswpout":
			switch {
			case strings.HasSuffix(m[0], "in"):
				Add(&md, "linux.mem."+strings.TrimSuffix(m[0], "in"), m[1], opentsdb.TagSet{"direction": "in"}, metadata.Counter, metadata.Page, "")
			case strings.HasSuffix(m[0], "out"):
				Add(&md, "linux.mem."+strings.TrimSuffix(m[0], "out"), m[1], opentsdb.TagSet{"direction": "out"}, metadata.Counter, metadata.Page, "")
			}
		case "pgfault", "pgmajfault":
			Add(&md, "linux.mem."+m[0], m[1], nil, metadata.Counter, metadata.Page, "")
		default:
			Add(&md, "linux.mem."+m[0], m[1], nil, metadata.Counter, metadata.None, "")
		}
		return nil
	}); err != nil {
		Error = err
	}
	num_cores := 0
	var t_util int
	if err := readLine("/proc/stat", func(s string) error {
		m := strings.Split(s, " ")
		if m == nil {
			return nil
		}
		switch {
		case strings.HasPrefix(m[0], "cpu"):
			tag_cpu := strings.TrimPrefix(m[0], "cpu")
			if tag_cpu != "" {
				num_cores++
			}
			for i, value := range m[1:] {
				if i >= len(cpu_fields) {
					break
				}
				tags := opentsdb.TagSet{
					"type": cpu_fields[i],
				}
				if tag_cpu != "" {
					tags["cpu"] = tag_cpu
					Add(&md, "linux.cpu.percpu", value, tags, metadata.Counter, metadata.CHz, cpu_stat_desc[i])
				} else {
					Add(&md, "linux.cpu", value, tags, metadata.Counter, metadata.CHz, cpu_stat_desc[i])
				}
			}
			if tag_cpu == "" {
				if len(m[1:]) < 3 {
					return nil
				}
				user, err := strconv.Atoi(m[1])
				if err != nil {
					return nil
				}
				nice, err := strconv.Atoi(m[2])
				if err != nil {
					return nil
				}
				system, err := strconv.Atoi(m[3])
				if err != nil {
					return nil
				}
				t_util = user + nice + system
			}
		case m[0] == "intr":
			Add(&md, "linux.intr", m[1], nil, metadata.Counter, metadata.Interupt, "")
		case m[0] == "ctxt":
			Add(&md, "linux.ctxt", m[1], nil, metadata.Counter, metadata.ContextSwitch, "")
		case m[0] == "processes":
			Add(&md, "linux.processes", m[1], nil, metadata.Counter, metadata.Process,
				"The number  of processes and threads created, which includes (but  is not limited  to) those  created by  calls to the  fork() and clone() system calls.")
		case m[0] == "procs_blocked":
			Add(&md, "linux.procs_blocked", m[1], nil, metadata.Gauge, metadata.Process, "The  number of  processes currently blocked, waiting for I/O to complete.")
		}
		return nil
	}); err != nil {
		Error = err
	}
	if num_cores != 0 && t_util != 0 {
		Add(&md, osCPU, t_util/num_cores, nil, metadata.Counter, metadata.Pct, "")
	}
	cpuinfo_index := 0
	if err := readLine("/proc/cpuinfo", func(s string) error {
		m := strings.Split(s, ":")
		if len(m) < 2 {
			return nil
		}
		m[0] = strings.TrimSpace(m[0])
		m[1] = strings.TrimSpace(m[1])
		if m[0] != "cpu MHz" {
			return nil
		}
		tags := opentsdb.TagSet{"cpu": strconv.Itoa(cpuinfo_index)}
		Add(&md, osCPUClock, m[1], tags, metadata.Gauge, metadata.MHz, osCPUClockDesc)
		Add(&md, "linux.cpu.clock", m[1], tags, metadata.Gauge, metadata.MHz, osCPUClockDesc)
		cpuinfo_index++
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
		if _, err := strconv.Atoi(irq_type); err == nil {
			if len(cols) == num_cpus+3 && strings.HasPrefix(cols[num_cpus+1], "IR-") {
				irq_type = cols[len(cols)-1]
			} else {
				// Interrupt type is just a number, ignore.
				return nil
			}
		} else {
			return nil
		}
		for i, val := range cols[1:] {
			if _, err := strconv.Atoi(val); i >= num_cpus || err != nil {
				// All values read, remaining cols contain textual description.
				break
			}
			Add(&md, "linux.interrupts", val, opentsdb.TagSet{"type": irq_type, "cpu": strconv.Itoa(i)}, metadata.Counter, metadata.Interupt, irq_type_desc[irq_type])
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
		ln++
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
