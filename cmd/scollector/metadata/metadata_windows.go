package metadata

import (
	"strings"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, metaWindowsVersion, metaWindowsIfaces)
}

func metaWindowsVersion() {
	util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return
		}
		AddMeta("", nil, "version", strings.Join(fields, " "), true)
	}, "cmd", "/c", "ver")
}

func metaWindowsIfaces() {
	var iface string
	util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		sp := strings.Split(line, ":")
		if len(fields) == 0 || len(sp) != 2 {
			return
		}
		if line[0] != ' ' {
			iface = sp[0]
		} else if strings.HasPrefix(line, "   IPv4 Address") {
			AddMeta("", opentsdb.TagSet{"iface": iface}, "addr", strings.TrimSpace(sp[1]), true)
		}
	}, "ipconfig")
}
