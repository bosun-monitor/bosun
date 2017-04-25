package collectors

import (
	"errors"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_meta_linux_version, Interval: time.Minute * 30})
	collectors = append(collectors, &IntervalCollector{F: c_meta_linux_serial, Interval: time.Minute * 30})
	collectors = append(collectors, &IntervalCollector{F: c_meta_linux_ifaces, Interval: time.Minute * 30})
}

func c_meta_linux_version() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	_ = util.ReadCommand(func(line string) error {
		metadata.AddMeta("", nil, "uname", line, true)
		return nil
	}, "uname", "-a")
	if !readOSRelease() {
		readIssue()
	}
	return md, nil
}

func readOSRelease() bool {
	var found bool
	_ = readLine("/etc/os-release", func(s string) error {
		fields := strings.SplitN(s, "=", 2)
		if len(fields) != 2 {
			return nil
		}
		if fields[0] == "PRETTY_NAME" {
			metadata.AddMeta("", nil, "version", strings.Trim(fields[1], `"`), true)
			found = true
		}
		return nil
	})
	return found
}

func readIssue() {
	_ = util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		hasNum := false
		for i := 0; i < len(fields); {
			if strings.HasPrefix(fields[i], `\`) {
				fields = append(fields[:i], fields[i+1:]...)
			} else {
				if v, _ := strconv.ParseFloat(fields[i], 32); v > 0 {
					hasNum = true
				}
				i++
			}
		}
		if !hasNum {
			return nil
		}
		metadata.AddMeta("", nil, "version", strings.Join(fields, " "), true)
		return nil
	}, "cat", "/etc/issue")
}

func c_meta_linux_serial() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	_ = util.ReadCommand(func(line string) error {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) != 2 {
			return nil
		}
		switch fields[0] {
		case "\tSerial Number":
			metadata.AddMeta("", nil, "serialNumber", strings.TrimSpace(fields[1]), true)
		case "\tManufacturer":
			metadata.AddMeta("", nil, "manufacturer", strings.TrimSpace(fields[1]), true)
		case "\tProduct Name":
			metadata.AddMeta("", nil, "model", strings.TrimSpace(fields[1]), true)
		}
		return nil
	}, "dmidecode", "-t", "system")
	return md, nil
}

var doneErr = errors.New("")

// metaLinuxIfacesMaster returns the bond master from s or "" if none exists.
func metaLinuxIfacesMaster(line string) string {
	sp := strings.Fields(line)
	for i := 4; i < len(sp); i += 2 {
		if sp[i-1] == "master" {
			return sp[i]
		}
	}
	return ""
}

func c_meta_linux_ifaces() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	metaIfaces(func(iface net.Interface, tags opentsdb.TagSet) {
		if speed, err := ioutil.ReadFile("/sys/class/net/" + iface.Name + "/speed"); err == nil {
			v, _ := strconv.Atoi(strings.TrimSpace(string(speed)))
			if v > 0 {
				const MbitToBit = 1e6
				metadata.AddMeta("", tags, "speed", v*MbitToBit, true)
			}
		}
		_ = util.ReadCommand(func(line string) error {
			if v := metaLinuxIfacesMaster(line); v != "" {
				metadata.AddMeta("", tags, "master", v, true)
				return doneErr
			}
			return nil
		}, "ip", "-o", "addr", "show", iface.Name)
	})
	return md, nil
}
