package collectors

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"bosun.org/_third_party/github.com/dimamedvedev/pstree"
)

var (
	testPSTree = &pstree.Tree{
		Procs: map[int]pstree.Process{
			1: {
				Name:     "init",
				Pid:      1,
				Parent:   0,
				Children: []int{20, 30},
			},
			20: {
				Name:     "supervisor1",
				Pid:      20,
				Parent:   1,
				Children: []int{21},
			},
			21: {
				Name:     "supervisor1-slave1",
				Pid:      21,
				Parent:   20,
				Children: nil,
			},
			30: {
				Name:     "supervisor1",
				Pid:      30,
				Parent:   1,
				Children: nil,
			},
		},
	}
	expectedWP = []*WatchedProc{
		{
			Command:  "test-service",
			Name:     "test-service",
			ArgMatch: regexp.MustCompile(""),
			idPool:   nil,
			Processes: map[string]int{
				"20": 1,
				"21": 2,
			},
		},
	}
	expEmpty = []*WatchedProc{}

	wildRe       = regexp.MustCompile(".*")
	singleRe     = regexp.MustCompile("test-service")
	wrongRe      = regexp.MustCompile("wrong")
	blackEmptyRe = regexp.MustCompile("^$")
)

const (
	testServicesPrefix = "./test/services_linux/"
)

func TestRunitAll(t *testing.T) {
	wp, _ := runitWatchedProc(testPSTree, testServicesPrefix, "etc/service", wildRe, blackEmptyRe)
	if !reflect.DeepEqual(expectedWP, wp) {
		t.Fail()
	}
}

func TestRunitSingle(t *testing.T) {
	wp, _ := runitWatchedProc(testPSTree, testServicesPrefix, "etc/service", singleRe, blackEmptyRe)
	if !reflect.DeepEqual(expectedWP, wp) {
		t.Fail()
	}
}
func TestRunitWrongRe(t *testing.T) {
	wp, _ := runitWatchedProc(testPSTree, testServicesPrefix, "etc/service", wrongRe, blackEmptyRe)
	if !reflect.DeepEqual(expEmpty, wp) {
		t.Fail()
	}

}

func TestInitAll(t *testing.T) {
	wp, _ := initdWatchedProc(testPSTree, testServicesPrefix, "var/run", wildRe, blackEmptyRe)
	if !reflect.DeepEqual(expectedWP, wp) {
		t.Fail()
	}

}

func TestInitSingle(t *testing.T) {
	wp, _ := initdWatchedProc(testPSTree, testServicesPrefix, "var/run", singleRe, blackEmptyRe)
	if !reflect.DeepEqual(expectedWP, wp) {
		t.Fail()
	}

}

func TestInitWrongRe(t *testing.T) {
	wp, _ := initdWatchedProc(testPSTree, testServicesPrefix, "var/run", wrongRe, blackEmptyRe)
	if !reflect.DeepEqual(expEmpty, wp) {
		t.Fail()
	}
}

type systemdListStub struct {
}

func (s systemdListStub) listUnits() ([]string, error) {
	units := []string{"test-service"}
	return units, nil
}

func (s systemdListStub) mainPid(serviceName string) (int, error) {
	if serviceName == "test-service" {
		return 20, nil
	}
	return 0, fmt.Errorf("pid not found for: %s", serviceName)
}

func TestSystemdAll(t *testing.T) {
	var systemdStub systemdListStub
	wp, _ := systemdWatchedProc(testPSTree, wildRe, blackEmptyRe, systemdStub)

	if !reflect.DeepEqual(expectedWP, wp) {
		t.Fail()
	}
}
