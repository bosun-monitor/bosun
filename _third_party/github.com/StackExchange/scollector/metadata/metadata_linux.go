package metadata

import (
	"strconv"
	"strings"

	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, metaLinuxVersion, metaLinuxIfaces)
}

func metaLinuxVersion() {
	util.ReadCommand(func(line string) error {
		AddMeta("", nil, "uname", line, true)
		return nil
	}, "uname", "-a")
	util.ReadCommand(func(line string) error {
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
		AddMeta("", nil, "version", strings.Join(fields, " "), true)
		return nil
	}, "cat", "/etc/issue")
}

func metaLinuxIfaces() {
	var iface string
	util.ReadCommand(func(line string) error {
		sp := strings.Fields(line)
		if len(sp) == 0 {
			iface = ""
			return nil
		}
		if iface == "" {
			iface = sp[0]
		}
		if iface == "lo" {
			return nil
		}
		if len(sp) > 1 && sp[0] == "inet" {
			asp := strings.Split(sp[1], ":")
			if len(asp) == 2 && asp[0] == "addr" {
				AddMeta("", opentsdb.TagSet{"iface": iface}, "addr", asp[1], true)
			}
		}
		return nil
	}, "ifconfig")
}
