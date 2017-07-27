package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func AddProcessConfig(params conf.ProcessParams) error {
	p, err := NewWatchedProc(params)
	if err != nil {
		return err
	}
	watchedProcs = append(watchedProcs, p)
	return nil
}

var watchedProcs = []*WatchedProc{}

var osPageSize = os.Getpagesize()

// linuxCoreCount counts the number of logical cpus since that is how cpu ticks
// are tracked
func linuxCoreCount() (c int64, err error) {
	if err = readLine("/proc/cpuinfo", func(s string) (err error) {
		f := strings.Fields(s)
		if len(f) > 2 && f[0] == "processor" {
			c++
			return
		}
		return
	}); err != nil {
		return c, fmt.Errorf("failed to read /proc/cpuinfo to get cpu core count: %v", err)
	}
	if c == 0 {
		return c, fmt.Errorf("got a core count of 0, expected at least one core")
	}
	return
}

func WatchProcesses() {
	if len(watchedProcs) == 0 {
		return
	}
	collectors = append(collectors, &IntervalCollector{
		F: func() (opentsdb.MultiDataPoint, error) {
			return c_linux_processes(watchedProcs)
		},
		name: "c_linux_processes",
	})
}

func linuxProcMonitor(w *WatchedProc, md *opentsdb.MultiDataPoint) error {
	var err error
	var processCount int
	var totalCPU int64
	var totalVirtualMem int64
	var totalRSSMem int64
	for proc, id := range w.Processes {
		pid := proc.Pid
		file_status, e := os.Stat("/proc/" + pid)
		if e != nil {
			w.Remove(proc)
			continue
		}
		processCount++
		stats_file, e := ioutil.ReadFile("/proc/" + pid + "/stat")
		if e != nil {
			w.Remove(proc)
			continue
		}
		io_file, e := ioutil.ReadFile("/proc/" + pid + "/io")
		if e != nil {
			w.Remove(proc)
			continue
		}
		limits, e := ioutil.ReadFile("/proc/" + pid + "/limits")
		if e != nil {
			w.Remove(proc)
			continue
		}
		fd_dir, e := os.Open("/proc/" + pid + "/fd")
		if e != nil {
			w.Remove(proc)
			continue
		}
		fds, e := fd_dir.Readdirnames(0)
		fd_dir.Close()
		if e != nil {
			w.Remove(proc)
			continue
		}
		stats := strings.Fields(string(stats_file))
		if len(stats) < 24 {
			err = fmt.Errorf("stats too short")
			continue
		}
		var io []string
		for _, line := range strings.Split(string(io_file), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 {
				io = append(io, f[1])
			}
		}
		if len(io) < 6 {
			err = fmt.Errorf("io too short")
			continue
		}
		tags := opentsdb.TagSet{"name": w.Name, "id": strconv.Itoa(id)}
		for _, line := range strings.Split(string(limits), "\n") {
			f := strings.Fields(line)
			if len(f) == 6 && strings.Join(f[0:3], " ") == "Max open files" {
				if f[3] != "unlimited" {
					Add(md, "linux.proc.num_fds_slim", f[3], tags, metadata.Gauge, metadata.Files, descLinuxSoftFileLimit)
					Add(md, "linux.proc.num_fds_hlim", f[4], tags, metadata.Gauge, metadata.Files, descLinuxHardFileLimit)
				}
			}
		}
		start_ts := file_status.ModTime().Unix()
		user, err := strconv.ParseInt(stats[13], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert process user cpu: %v", err)
		}
		sys, err := strconv.ParseInt(stats[14], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert process system cpu: %v", err)
		}
		totalCPU += user + sys
		Add(md, "linux.proc.cpu", stats[13], opentsdb.TagSet{"type": "user"}.Merge(tags), metadata.Counter, metadata.Pct, descLinuxProcCPUUser)
		Add(md, "linux.proc.cpu", stats[14], opentsdb.TagSet{"type": "system"}.Merge(tags), metadata.Counter, metadata.Pct, descLinuxProcCPUSystem)
		Add(md, "linux.proc.mem.fault", stats[9], opentsdb.TagSet{"type": "minflt"}.Merge(tags), metadata.Counter, metadata.Fault, descLinuxProcMemFaultMin)
		Add(md, "linux.proc.mem.fault", stats[11], opentsdb.TagSet{"type": "majflt"}.Merge(tags), metadata.Counter, metadata.Fault, descLinuxProcMemFaultMax)
		virtual, err := strconv.ParseInt(stats[22], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert process virtual memory: %v", err)
		}
		totalVirtualMem += virtual
		rss, err := strconv.ParseInt(stats[23], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert process rss memory: %v", err)
		}
		if pid == string(os.Getpid()) {
			TotalScollectorMemoryMB = uint64(rss) * uint64(osPageSize) / 1024 / 1024
		}
		totalRSSMem += rss
		Add(md, "linux.proc.mem.virtual", stats[22], tags, metadata.Gauge, metadata.Bytes, descLinuxProcMemVirtual)
		Add(md, "linux.proc.mem.rss", stats[23], tags, metadata.Gauge, metadata.Page, descLinuxProcMemRss)
		Add(md, "linux.proc.mem.rss_bytes", rss*int64(osPageSize), tags, metadata.Gauge, metadata.Bytes, descLinuxProcMemRssBytes)
		Add(md, "linux.proc.char_io", io[0], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcCharIoRead)
		Add(md, "linux.proc.char_io", io[1], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcCharIoWrite)
		Add(md, "linux.proc.syscall", io[2], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Syscall, descLinuxProcSyscallRead)
		Add(md, "linux.proc.syscall", io[3], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Syscall, descLinuxProcSyscallWrite)
		Add(md, "linux.proc.io_bytes", io[4], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcIoBytesRead)
		Add(md, "linux.proc.io_bytes", io[5], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcIoBytesWrite)
		Add(md, "linux.proc.num_fds", len(fds), tags, metadata.Gauge, metadata.Files, descLinuxProcFd)
		Add(md, "linux.proc.start_time", start_ts, tags, metadata.Gauge, metadata.Timestamp, descLinuxProcStartTS)
		Add(md, "linux.proc.uptime", now()-start_ts, tags, metadata.Gauge, metadata.Second, descLinuxProcUptime)
		Add(md, "linux.proc.pid", pid, tags, metadata.Gauge, metadata.Unit("PID"), osProcPID)
	}
	coreCount, err := linuxCoreCount()
	if err != nil {
		return fmt.Errorf("failed to get core count: %v", err)
	}
	tsName := opentsdb.TagSet{"name": w.Name}
	if processCount > 0 {
		Add(md, osProcCPU, float64(totalCPU)/float64(coreCount), tsName, metadata.Counter, metadata.Pct, osProcCPUDesc)
		Add(md, osProcMemReal, totalRSSMem*int64(os.Getpagesize()), tsName, metadata.Gauge, metadata.Bytes, osProcMemRealDesc)
		Add(md, osProcMemVirtual, totalVirtualMem, tsName, metadata.Gauge, metadata.Bytes, osProcMemVirtualDesc)
		Add(md, osProcCount, processCount, tsName, metadata.Gauge, metadata.Process, osProcCountDesc)
	}
	if w.IncludeCount {
		Add(md, "linux.proc.count", processCount, tsName, metadata.Gauge, metadata.Process, descLinuxProcCount)
	}
	return err
}

const (
	descLinuxProcCPUUser      = "The amount of time that this process has been scheduled in user mode."
	descLinuxProcCPUSystem    = "The amount of time that this process has been scheduled in kernel mode"
	descLinuxProcMemFaultMin  = "The number of minor faults the process has made which have not required loading a memory page from disk."
	descLinuxProcMemFaultMax  = "The number of major faults the process has made which have required loading a memory page from disk."
	descLinuxProcMemVirtual   = "The virtual memory size."
	descLinuxProcMemRss       = "The resident set size (number of pages the process has in real memory including shared pages)."
	descLinuxProcMemRssBytes  = "The resident set size (number of bytes the process has in real memory including shared pages)."
	descLinuxProcCharIoRead   = "The number of bytes which this task has caused to be read from storage. This is simply the sum of bytes which this process passed to read(2) and similar system calls. It includes things such as terminal I/O and is unaffected by whether or not actual physical disk I/O was required (the read might have been satisfied from pagecache)"
	descLinuxProcCharIoWrite  = "The number of bytes which this task has caused, or shall cause to be written to disk. Similar caveats apply here as with read."
	descLinuxProcSyscallRead  = "An attempt to count the number of read I/O operations—that is, system calls such as read(2) and pread(2)."
	descLinuxProcSyscallWrite = "Attempt to count the number of write I/O operations—that is, system calls such as write(2) and pwrite(2)."
	descLinuxProcIoBytesRead  = "An attempt to count the number of bytes which this process really did cause to be fetched from the storage layer. This is accurate for block-backed filesystems."
	descLinuxProcIoBytesWrite = "An Attempt to count the number of bytes which this process caused to be sent to the storage layer."
	descLinuxProcFd           = "The number of open file descriptors."
	descLinuxSoftFileLimit    = "The soft limit on the number of open file descriptors."
	descLinuxHardFileLimit    = "The hard limit on the number of open file descriptors."
	descLinuxProcUptime       = "The length of time, in seconds, since the process was started."
	descLinuxProcStartTS      = "The timestamp of process start."
	descLinuxProcCount        = "The number of currently running processes."
)

type byModTime []os.FileInfo

func (bmt byModTime) Len() int      { return len(bmt) }
func (bmt byModTime) Swap(i, j int) { bmt[i], bmt[j] = bmt[j], bmt[i] }
func (bmt byModTime) Less(i, j int) bool {
	// If the creation times are identical, sort by filename (pid) instead.
	if bmt[i].ModTime() == bmt[j].ModTime() {
		return sort.StringsAreSorted([]string{bmt[i].Name(), bmt[j].Name()})
	}
	return bmt[i].ModTime().UnixNano() < bmt[j].ModTime().UnixNano()
}

func getLinuxProccesses() ([]*Process, error) {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	sort.Sort(byModTime(files))
	var pidFiles []os.FileInfo
	for _, f := range files {
		if _, err := strconv.Atoi(f.Name()); err == nil && f.IsDir() {
			pidFiles = append(pidFiles, f)
		}
	}
	var lps []*Process
	for _, pidFile := range pidFiles {
		cl, err := getLinuxCmdline(pidFile.Name())
		if err != nil || cl == nil {
			//Continue because the pid might not exist any more
			continue
		}
		lp := &Process{
			Pid:     pidFile.Name(),
			Command: cl[0],
			Started: pidFile.ModTime(),
		}
		if len(cl) > 1 {
			lp.Arguments = strings.Join(cl[1:], "")
		}
		lps = append(lps, lp)
	}
	return lps, nil
}

func getLinuxCmdline(pid string) ([]string, error) {
	cmdline, err := ioutil.ReadFile("/proc/" + pid + "/cmdline")
	if err != nil {
		return nil, err
	}
	cl := strings.Split(string(cmdline), "\x00")
	if len(cl) < 1 || len(cl[0]) == 0 {
		return nil, nil
	}
	return cl, nil
}

func c_linux_processes(procs []*WatchedProc) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	lps, err := getLinuxProccesses()
	if err != nil {
		return nil, nil
	}
	for _, w := range procs {
		w.Check(lps)
		if e := linuxProcMonitor(w, &md); e != nil {
			err = e
		}
	}
	return md, err
}

type Process struct {
	Pid       string
	Command   string
	Arguments string
	Started   time.Time
}

// NewWatchedProc takes a configuration block [[Process]] from conf
func NewWatchedProc(params conf.ProcessParams) (*WatchedProc, error) {
	if params.Name == "" {
		params.Name = params.Command
	}
	if !opentsdb.ValidTSDBString(params.Name) {
		return nil, fmt.Errorf("bad process name: %v", params.Name)
	}
	return &WatchedProc{
		Command:      regexp.MustCompile(params.Command),
		Name:         params.Name,
		IncludeCount: params.IncludeCount,
		Processes:    make(map[Process]int),
		ArgMatch:     regexp.MustCompile(params.Args),
		idPool:       new(idPool),
	}, nil
}

type WatchedProc struct {
	Command      *regexp.Regexp
	Name         string
	IncludeCount bool
	Processes    map[Process]int
	ArgMatch     *regexp.Regexp
	*idPool
}

// Check finds all matching processes and assigns them a new unique id. If
// WatchedProc has processes that no longer exist, it removes them from
// WatchedProc.Processes.
func (w *WatchedProc) Check(procs []*Process) {
	procFound := make(map[Process]bool)
	for _, l := range procs {
		if _, ok := w.Processes[*l]; ok {
			procFound[*l] = true
			continue
		}
		if !w.Command.MatchString(l.Command) {
			continue
		}
		if !w.ArgMatch.MatchString(l.Arguments) {
			continue
		}
		w.Processes[*l] = w.get()
		procFound[*l] = true
	}
	for proc := range w.Processes {
		if !procFound[proc] {
			w.Remove(proc)
		}
	}
}

func (w *WatchedProc) Remove(proc Process) {
	w.put(w.Processes[proc])
	delete(w.Processes, proc)
}

type idPool struct {
	free []int
	next int
}

func (i *idPool) get() int {
	if len(i.free) == 0 {
		i.next++
		return i.next
	}
	sort.Ints(i.free)

	var newId int
	newId, i.free = i.free[0], i.free[1:]

	return newId
}

func (i *idPool) put(v int) {
	i.free = append(i.free, v)
}

// InContainer detects if a process is running in a Linux container.
func InContainer(pid string) bool {
	pidNameSpaceFile := fmt.Sprintf("/proc/%v/ns/pid", pid)
	if pidNameSpace, err := os.Readlink(pidNameSpaceFile); err == nil {
		if initNameSpace, err := os.Readlink("/proc/1/ns/pid"); err == nil {
			return initNameSpace != pidNameSpace
		}
	}
	return false
}
