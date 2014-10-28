package metadata

import (
	"strings"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
	"github.com/StackExchange/slog"
	"github.com/StackExchange/wmi"
)

func init() {
	metafuncs = append(metafuncs, metaWindowsVersion, metaWindowsIfaces)
}

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) error {
	return wmi.QueryNamespace(query, dst, namespace)
}

func metaWindowsVersion() {
	var dst []Win32_OperatingSystem
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		slog.Error(err)
		return
	} else {
		for _, v := range dst {
			AddMeta("", nil, "version", v.Version, true)
			AddMeta("", nil, "versionCaption", v.Caption, true)
		}
	}
}

type Win32_OperatingSystem struct {
	Caption         string
	CurrentTimeZone int16
	InstallDate     string
	Organization    string
	OSArchitecture  string
	OSLanguage      int32
	Version         string
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
