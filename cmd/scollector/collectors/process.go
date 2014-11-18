package collectors

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/opentsdb"
)

type Process struct {
	Pid       string
	Command   string
	Arguments string
}

// NewWatchedProc takes a string of the form "command,name,regex".
func NewWatchedProc(watch string) (*WatchedProc, error) {
	sp := strings.SplitN(watch, ",", 3)
	if len(sp) != 3 {
		return nil, fmt.Errorf("watched proc requires three fields")
	}
	if sp[1] == "" {
		sp[1] = sp[0]
	}
	if !opentsdb.ValidTag(sp[1]) {
		return nil, fmt.Errorf("bad process name: %v", sp[1])
	}
	return &WatchedProc{
		Command:   sp[0],
		Name:      sp[1],
		Processes: make(map[string]int),
		ArgMatch:  regexp.MustCompile(sp[2]),
		idPool:    new(idPool),
	}, nil
}

type WatchedProc struct {
	Command   string
	Name      string
	Processes map[string]int
	ArgMatch  *regexp.Regexp
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
