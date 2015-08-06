// package pstree provides an API to retrieve the process tree of a given
// process-id.
// Modified for usage with Bosun http://bosun.org

package pstree

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// New returns the whole system process tree.
func New() (*Tree, error) {
	return NewFromDir("/proc")
}

// NewFromDir allows to test from different directory
func NewFromDir(procPath string) (*Tree, error) {
	files, err := filepath.Glob(procPath + "/[0-9]*")
	if err != nil {
		return nil, err
	}

	procs := make(map[int]Process, len(files))
	for _, dir := range files {
		proc, err := scan(dir)
		if err != nil {
			return nil, err
		}
		if proc.Pid == 0 {
			// process vanished since Glob.
			continue
		}
		procs[proc.Pid] = proc
	}

	for pid, proc := range procs {
		if proc.Parent == 0 {
			continue
		}
		parent, ok := procs[proc.Parent]
		if !ok {
			log.Panicf(
				"internal logic error. parent of [%d] does not exist!",
				pid,
			)
		}
		parent.Children = append(parent.Children, pid)
		procs[parent.Pid] = parent
	}

	for pid, proc := range procs {
		if len(proc.Children) > 0 {
			sort.Ints(proc.Children)
		}
		procs[pid] = proc
	}

	tree := &Tree{
		Procs: procs,
	}
	return tree, err
}

func scan(dir string) (Process, error) {
	process := Process{}
	fileStat, err := os.Stat(dir + "/status")
	if err != nil {
		// process vanished
		//		return Process{}, nil
		return Process{}, err
	}
	process.StartTime = fileStat.ModTime().Unix()
	contents, err := ioutil.ReadFile(dir + "/status")
	if err != nil {
		//		return Process{}, err
		return Process{}, err
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		tabParts := strings.SplitN(line, "\t", 2)
		if len(tabParts) < 2 {
			continue
		}
		value := tabParts[1]
		switch strings.TrimRight(tabParts[0], ":") {
		case "Name":
			process.Name = strings.Trim(value, " \t")
		case "Pid":
			pid, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return Process{}, err
			}
			process.Pid = int(pid)
		case "PPid":
			ppid, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return Process{}, err
			}
			process.Parent = int(ppid)
		}

	}
	return process, nil
}

// Tree is a tree of processes.
type Tree struct {
	Procs map[int]Process
}

// Process stores informations about a UNIX process
type Process struct {
	Name      string
	Pid       int
	Parent    int
	Children  []int
	StartTime int64
}

type byModTime []Process

func (bmt byModTime) Len() int      { return len(bmt) }
func (bmt byModTime) Swap(i, j int) { bmt[i], bmt[j] = bmt[j], bmt[i] }
func (bmt byModTime) Less(i, j int) bool {
	return bmt[i].StartTime < bmt[j].StartTime
}

// Children returns a slice of all children processes
func (t Tree) Children(pid int) []Process {
	subTree := []Process{t.Procs[pid]}

	switch t.Procs[pid].Children {
	case nil:
		return subTree

	}
	for _, p := range t.Procs[pid].Children {
		subs := t.Children(p)
		subTree = append(subTree, subs...)
	}
	return subTree
}

// SubTreeMapID returns a map of all children processes with ID's assigned by creation time
func (t Tree) SubTreeMapID(pid int) map[string]int {
	mapWithID := make(map[string]int)
	subTree := t.Children(pid)

	sort.Sort(byModTime(subTree))
	for i, proc := range subTree {

		mapWithID[strconv.Itoa(proc.Pid)] = i + 1
	}
	return mapWithID
}
