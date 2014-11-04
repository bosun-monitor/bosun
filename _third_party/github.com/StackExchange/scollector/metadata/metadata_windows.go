package metadata

import (
	"strings"

	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, metaWindowsVersion, metaWindowsIfaces)
}

func metaWindowsVersion() {
	util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return nil
		}
		AddMeta("", nil, "version", strings.Join(fields, " "), true)
		return nil
	}, "cmd", "/c", "ver")
}

func metaWindowsIfaces() {
	var iface string
	util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		sp := strings.Split(line, ":")
		if len(fields) == 0 || len(sp) != 2 {
			return nil
		}
		if line[0] != ' ' {
			iface = sp[0]
		} else if strings.HasPrefix(line, "   IPv4 Address") {
			AddMeta("", opentsdb.TagSet{"iface": iface}, "addr", strings.TrimSpace(sp[1]), true)
		}
		return nil
	}, "ipconfig")
}
