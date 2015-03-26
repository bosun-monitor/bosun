package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

type crSummary struct {
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time"`
	ElapsedTime float32 `json:"elapsed_time"`
	Status      string  `json:"status"`
}

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_chef_linux, Enable: chefEnable})
}

const (
	chefRunLog = "/var/chef/reports/run_status_json.log"
)

func chefEnable() bool {
	_, err := os.Stat(chefRunLog)
	return err == nil
}

func c_chef_linux() (opentsdb.MultiDataPoint, error) {
	md, err := chef_linux(chefRunLog)
	if err != nil {
		return nil, err
	}
	return md, nil
}

func chef_linux(logfile string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var cr crSummary
	// Gather stats from the run summary
	s, err := readLastLine(chefRunLog, 4096)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(s), &cr)
	if err != nil {
		return nil, err
	}

	endTime, err := time.Parse("2006-01-02 15:04:05 -0700", cr.EndTime)
	if err != nil {
		return nil, err
	}

	switch cr.Status {
	case "success":
		AddTS(&md, "chef.run_status", endTime.Unix(), 0, nil, metadata.Gauge, metadata.Bool, descChefRunStatus)
	case "failed":
		AddTS(&md, "chef.run_status", endTime.Unix(), 1, nil, metadata.Gauge, metadata.Bool, descChefRunStatus)
	default:
		err := fmt.Errorf("bad chef run status: %s", cr.Status)
		return nil, err
	}
	AddTS(&md, "chef.run_time", endTime.Unix(), cr.ElapsedTime, nil, metadata.Gauge, metadata.Second, descChefRunTime)

	lastrun_delay := time.Now().Sub(endTime).Seconds()
	Add(&md, "chef.lastrun_delay", lastrun_delay, nil, metadata.Gauge, metadata.Second, descChefLastRunDelay)

	return md, nil
}

const (
	descChefRunStatus    = "Status of Chef run."
	descChefRunTime      = "Time which Chef took to run."
	descChefLastRunDelay = "Time passed after last Chef run."
)

func readLastLines(filepath string, count int, offset int64) ([]string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size < offset {
		offset = size
	}

	buf := make([]byte, offset)
	if _, err := f.ReadAt(buf, size-offset); err != nil && err != io.EOF {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(buf[:])))

	scanner.Split(bufio.ScanLines)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	to := len(lines)
	from := len(lines) - count

	if from < 0 {
		from = 0
	}

	return lines[from:to], nil
}

func readLastLine(filepath string, offset int64) (string, error) {
	res, err := readLastLines(filepath, 1, offset)

	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		err := fmt.Errorf("wrong slice size")
		return "", err
	}
	return res[0], nil
}
