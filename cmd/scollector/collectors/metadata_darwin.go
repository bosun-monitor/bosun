package collectors

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_meta_darwin_version, Interval: time.Minute * 30})
	collectors = append(collectors, &IntervalCollector{F: c_meta_darwin_interfaces, Interval: time.Minute * 30})
}

func c_meta_darwin_version() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	util.ReadCommand(func(line string) error {
		metadata.AddMeta("", nil, "uname", line, true)
		return nil
	}, "uname", "-a")
	var name, vers, build string
	util.ReadCommand(func(line string) error {
		sp := strings.SplitN(line, ":", 2)
		if len(sp) != 2 {
			return nil
		}
		v := strings.TrimSpace(sp[1])
		switch sp[0] {
		case "ProductName":
			name = v
		case "ProductVersion":
			vers = v
		case "BuildVersion":
			build = v
		}
		return nil
	}, "sw_vers")
	if name != "" && vers != "" && build != "" {
		metadata.AddMeta("", nil, "version", fmt.Sprintf("%s.%s", vers, build), true)
		metadata.AddMeta("", nil, "versionCaption", fmt.Sprintf("%s %s", name, vers), true)
	}
	return md, nil
}

func c_meta_darwin_interfaces() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	metaIfaces(nil)
	return md, nil
}
