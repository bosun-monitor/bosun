package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"bosun.org/_third_party/github.com/dimamedvedev/pstree"
	"bosun.org/util"

	"bosun.org/opentsdb"
)

const (
	runitServicesDir = "/etc/service"
)

func RunitServices(whiteList, blackList string) error {
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return runitServices(whiteList, blackList)
			},
			name: fmt.Sprintf("runit-%s-%s", whiteList, blackList),
		},
	)
	return nil
}

func InitdServices(whiteList, blackList string) error {
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return initdServices(whiteList, blackList)
			},
			name: fmt.Sprintf("initd-%s-%s", whiteList, blackList),
		},
	)
	return nil
}

func SystemdServices(whiteList, blackList string) error {
	collectors = append(collectors,
		&IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return systemdServices(whiteList, blackList)
			},
			name: fmt.Sprintf("systemd-%s-%s", whiteList, blackList),
		},
	)
	return nil
}

func runitServices(whiteList, blackList string) (opentsdb.MultiDataPoint, error) {
	whiteRe, err := regexp.Compile(whiteList)
	if err != nil {
		return nil, err
	}
	if blackList == "" {
		blackList = "^$"
	}
	blackRe, err := regexp.Compile(blackList)
	if err != nil {
		return nil, err
	}
	psTree, err := pstree.New()
	if err != nil {
		return nil, err
	}

	wps, err := runitWatchedProc(psTree, "/", "/etc/service", whiteRe, blackRe)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, wp := range wps {
		if e := linuxProcMonitor(wp, &md); e != nil {
			return nil, e
		}
	}
	return md, nil
}

func initdServices(whiteList, blackList string) (opentsdb.MultiDataPoint, error) {
	whiteRe, err := regexp.Compile(whiteList)
	if err != nil {
		return nil, err
	}
	if blackList == "" {
		blackList = "^$"
	}
	blackRe, err := regexp.Compile(blackList)
	if err != nil {
		return nil, err
	}
	psTree, err := pstree.New()
	if err != nil {
		return nil, err
	}
	wps, err := initdWatchedProc(psTree, "/", "var/run", whiteRe, blackRe)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, wp := range wps {
		if e := linuxProcMonitor(wp, &md); e != nil {
			return nil, e
		}
	}
	return md, nil
}

func systemdServices(whiteList, blackList string) (opentsdb.MultiDataPoint, error) {
	whiteRe, err := regexp.Compile(whiteList)
	if err != nil {
		return nil, err
	}
	if blackList == "" {
		blackList = "^$"
	}
	blackRe, err := regexp.Compile(blackList)
	if err != nil {
		return nil, err
	}
	psTree, err := pstree.New()
	if err != nil {
		return nil, err
	}
	wps, err := systemdWatchedProc(psTree, whiteRe, blackRe, systemdL)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, wp := range wps {
		if e := linuxProcMonitor(wp, &md); e != nil {
			return nil, e
		}
	}
	return md, nil
}

// ReadPid tries to find and read pidfile into an int
func ReadPid(prefix string, filepaths ...string) (int, error) {
	for _, filepath := range filepaths {
		content, err := ioutil.ReadFile(filepath)
		if err != nil {
			continue
		}
		trimmed := strings.Trim(string(content[:]), "\n")
		_, err = os.Stat(prefix + "proc/" + trimmed)
		if err != nil {
			continue
		}
		pid, err := strconv.ParseInt(trimmed, 10, 32)
		if err != nil {
			continue
		}
		return int(pid), nil
	}
	return 0, fmt.Errorf("pid does not exist\n")
}

func runitWatchedProc(psTree *pstree.Tree, prefix string, svcDir string, whiteRe, blackRe *regexp.Regexp) ([]*WatchedProc, error) {
	matches, err := filepath.Glob(prefix + svcDir + "/*/supervise/pid")
	if err != nil {

		return nil, err
	}
	wp := []*WatchedProc{}
	for _, match := range matches {
		pid, err := ReadPid(prefix, match)
		if err != nil {
			fmt.Printf("Err: %v", err)
			continue
		}
		matchSplit := strings.Split(match, "/")
		// '/etc/service/name/supervise/pid'
		name := matchSplit[len(matchSplit)-3]
		if !whiteRe.MatchString(name) || blackRe.MatchString(name) {
			continue
		}
		wp = append(wp,
			&WatchedProc{
				Command:   name,
				Name:      name,
				Processes: psTree.SubTreeMapID(pid),
				ArgMatch:  regexp.MustCompile(""),
				idPool:    nil,
			},
		)
	}
	return wp, nil
}

func initdWatchedProc(psTree *pstree.Tree, prefix string, svcDir string, whiteRe, blackRe *regexp.Regexp) ([]*WatchedProc, error) {
	pidPaths := []string{}
	searchDir, err := filepath.EvalSymlinks(prefix + svcDir)
	if err != nil {
		return nil, err
	}
	errWalk := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".pid") {
			pidPaths = append(pidPaths, path)
		}
		return nil
	})
	if errWalk != nil {
		return nil, errWalk
	}
	wp := []*WatchedProc{}
	for _, pidPath := range pidPaths {
		filename := filepath.Base(pidPath)
		name := strings.TrimSuffix(filename, ".pid")
		if !whiteRe.MatchString(name) || blackRe.MatchString(name) {
			continue
		}
		pid, err := ReadPid(prefix, pidPath)
		if err != nil {
			continue
		}
		wp = append(wp,
			&WatchedProc{
				Command:   name,
				Name:      name,
				Processes: psTree.SubTreeMapID(pid),
				ArgMatch:  regexp.MustCompile(""),
				idPool:    nil,
			},
		)
	}
	return wp, nil
}

func systemdWatchedProc(psTree *pstree.Tree, whiteRe, blackRe *regexp.Regexp, systemd systemdLister) ([]*WatchedProc, error) {
	units, err := systemd.listUnits()
	if err != nil {
		return nil, err
	}
	wp := []*WatchedProc{}
	for _, name := range units {
		if !whiteRe.MatchString(name) || blackRe.MatchString(name) {
			continue
		}
		pid, err := systemd.mainPid(name)
		if err != nil {
			continue
		}
		wp = append(wp,
			&WatchedProc{
				Command:   name,
				Name:      name,
				Processes: psTree.SubTreeMapID(pid),
				ArgMatch:  regexp.MustCompile(""),
				idPool:    nil,
			},
		)

	}
	return wp, nil
}

type systemdLister interface {
	listUnits() ([]string, error)
	mainPid(string) (int, error)
}

type systemdList struct {
}

var systemdL systemdList

func (s systemdList) listUnits() ([]string, error) {
	units := []string{}
	_ = util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil
		}
		name := fields[0]
		if !strings.Contains(fields[0], ".service") {
			return nil
		}
		units = append(units, strings.TrimSuffix(name, ".service"))
		return nil
	}, "systemctl", "list-units", "--type=service")
	return units, nil
}

func (s systemdList) mainPid(unitName string) (int, error) {
	var pid int
	_ = util.ReadCommand(func(line string) error {
		fields := strings.Split(line, "=")
		if fields[0] != "MainPID" {
			return fmt.Errorf("string must contain 'MainPID='")
		}
		p, err := strconv.ParseInt(fields[1], 10, 32)
		if err != nil {
			return err
		}
		pid = int(p)
		return nil
	}, "systemctl", "show", unitName, "--property=MainPID")
	return pid, nil
}
