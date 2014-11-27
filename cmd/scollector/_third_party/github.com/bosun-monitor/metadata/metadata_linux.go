package metadata

import (
	"io/ioutil"
	"net"
	"strconv"
	"strings"

	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/opentsdb"
	"github.com/bosun-monitor/scollector/_third_party/github.com/bosun-monitor/util"
)

func init() {
	metafuncs = append(metafuncs, metaLinuxVersion, metaLinuxIfaces)
}

func metaLinuxVersion() {
	_ = util.ReadCommand(func(line string) error {
		AddMeta("", nil, "uname", line, true)
		return nil
	}, "uname", "-a")
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
		AddMeta("", nil, "version", strings.Join(fields, " "), true)
		return nil
	}, "cat", "/etc/issue")
}

func metaLinuxIfaces() {
	metaIfaces(func(iface net.Interface, tags opentsdb.TagSet) {
		speed, err := ioutil.ReadFile("/sys/class/net/" + iface.Name + "/speed")
		if err != nil {
			return
		}
		v, _ := strconv.Atoi(strings.TrimSpace(string(speed)))
		if v > 0 {
			const MbitToBit = 1e6
			AddMeta("", tags, "speed", v*MbitToBit, true)
		}
	})
}
