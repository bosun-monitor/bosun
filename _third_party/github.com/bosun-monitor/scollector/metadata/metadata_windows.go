package metadata

import (
	"strings"

	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/slog"
	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/wmi"
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/scollector/opentsdb"
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, metaWindowsVersion, metaWindowsIfaces)
}

func metaWindowsVersion() {
	var dst []Win32_OperatingSystem
	q := wmi.CreateQuery(&dst, "")
	err := wmi.Query(q, &dst)
	if err != nil {
		slog.Error(err)
		return
	}
	for _, v := range dst {
		AddMeta("", nil, "version", v.Version, true)
		AddMeta("", nil, "versionCaption", v.Caption, true)
	}
}

type Win32_OperatingSystem struct {
	Caption string
	Version string
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
