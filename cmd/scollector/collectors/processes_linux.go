package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

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
	for pid, id := range w.Processes {
		file_status, e := os.Stat("/proc/" + pid)
		if e != nil {
			w.Remove(pid)
			continue
		}
		processCount++
		stats_file, e := ioutil.ReadFile("/proc/" + pid + "/stat")
		if e != nil {
			w.Remove(pid)
			continue
		}
		io_file, e := ioutil.ReadFile("/proc/" + pid + "/io")
		if e != nil {
			w.Remove(pid)
			continue
		}
		limits, e := ioutil.ReadFile("/proc/" + pid + "/limits")
		if e != nil {
			w.Remove(pid)
			continue
		}
		fd_dir, e := os.Open("/proc/" + pid + "/fd")
		if e != nil {
			w.Remove(pid)
			continue
		}
		fds, e := fd_dir.Readdirnames(0)
		fd_dir.Close()
		if e != nil {
			w.Remove(pid)
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
		Add(md, "linux.proc.cpu", stats[13], opentsdb.TagSet{"type": "user"}.Merge(tags), metadata.Counter, metadata.Pct, descLinuxProcCPUUser)
		Add(md, "linux.proc.cpu", stats[14], opentsdb.TagSet{"type": "system"}.Merge(tags), metadata.Counter, metadata.Pct, descLinuxProcCPUSystem)
		Add(md, "linux.proc.mem.fault", stats[9], opentsdb.TagSet{"type": "minflt"}.Merge(tags), metadata.Counter, metadata.Fault, descLinuxProcMemFaultMin)
		Add(md, "linux.proc.mem.fault", stats[11], opentsdb.TagSet{"type": "majflt"}.Merge(tags), metadata.Counter, metadata.Fault, descLinuxProcMemFaultMax)
		Add(md, "linux.proc.mem.virtual", stats[22], tags, metadata.Gauge, metadata.Bytes, descLinuxProcMemVirtual)
		Add(md, "linux.proc.mem.rss", stats[23], tags, metadata.Gauge, metadata.Page, descLinuxProcMemRss)
		Add(md, "linux.proc.char_io", io[0], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcCharIoRead)
		Add(md, "linux.proc.char_io", io[1], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcCharIoWrite)
		Add(md, "linux.proc.syscall", io[2], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Syscall, descLinuxProcSyscallRead)
		Add(md, "linux.proc.syscall", io[3], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Syscall, descLinuxProcSyscallWrite)
		Add(md, "linux.proc.io_bytes", io[4], opentsdb.TagSet{"type": "read"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcIoBytesRead)
		Add(md, "linux.proc.io_bytes", io[5], opentsdb.TagSet{"type": "write"}.Merge(tags), metadata.Counter, metadata.Bytes, descLinuxProcIoBytesWrite)
		Add(md, "linux.proc.num_fds", len(fds), tags, metadata.Gauge, metadata.Files, descLinuxProcFd)
		Add(md, "linux.proc.start_time", start_ts, tags, metadata.Gauge, metadata.Timestamp, descLinuxProcStartTS)
		Add(md, "linux.proc.uptime", now()-start_ts, tags, metadata.Gauge, metadata.Second, descLinuxProcUptime)
	}
	if w.IncludeCount {
		Add(md, "linux.proc.count", processCount, opentsdb.TagSet{"name": w.Name}, metadata.Gauge, metadata.Process, descLinuxProcCount)
	}
	return err
}

const (
	descLinuxProcCPUUser      = "The amount of time that this process has been scheduled in user mode."
	descLinuxProcCPUSystem    = "The amount of time that this process has been scheduled in kernel mode"
	descLinuxProcMemFaultMin  = "The number of minor faults the process has made which have not required loading a memory page from disk."
	descLinuxProcMemFaultMax  = "The number of major faults the process has made which have required loading a memory page from disk."
	descLinuxProcMemVirtual   = "The virtual memory size."
	descLinuxProcMemRss       = "The resident set size (number of pages the process has in real memory."
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
	return bmt[i].ModTime().Unix() < bmt[j].ModTime().Unix()
}

func getLinuxProccesses() ([]*Process, error) {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	sort.Sort(byModTime(files))
	var pids []string
	for _, f := range files {
		if _, err := strconv.Atoi(f.Name()); err == nil && f.IsDir() {
			pids = append(pids, f.Name())
		}
	}
	var lps []*Process
	for _, pid := range pids {
		cl, err := getLinuxCmdline(pid)
		if err != nil || cl == nil {
			//Continue because the pid might not exist any more
			continue
		}
		lp := &Process{
			Pid:     pid,
			Command: cl[0],
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
}

// NewWatchedProc takes a configuration block [[Process]] from conf
func NewWatchedProc(params conf.ProcessParams) (*WatchedProc, error) {
	if params.Name == "" {
		params.Name = params.Command
	}
	if !opentsdb.ValidTag(params.Name) {
		return nil, fmt.Errorf("bad process name: %v", params.Name)
	}
	return &WatchedProc{
		Command:      params.Command,
		Name:         params.Name,
		IncludeCount: params.IncludeCount,
		Processes:    make(map[string]int),
		ArgMatch:     regexp.MustCompile(params.Args),
		idPool:       new(idPool),
	}, nil
}

type WatchedProc struct {
	Command      string
	Name         string
	IncludeCount bool
	Processes    map[string]int
	ArgMatch     *regexp.Regexp
	*idPool
}

// Check finds all matching processes and assigns them a new unique id.
func (w *WatchedProc) Check(procs []*Process) {
	for _, l := range procs {
		if _, ok := w.Processes[l.Pid]; ok {
			continue
		}
		if !strings.Contains(l.Command, w.Command) {
			continue
		}
		if !w.ArgMatch.MatchString(l.Arguments) {
			continue
		}
		w.Processes[l.Pid] = w.get()
	}
}

func (w *WatchedProc) Remove(pid string) {
	w.put(w.Processes[pid])
	delete(w.Processes, pid)
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
	return i.free[0]
}

func (i *idPool) put(v int) {
	i.free = append(i.free, v)
}
