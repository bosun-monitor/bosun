package metadata

import (
	"strings"

	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, collectMetadataOmreport)
}

func collectMetadataOmreport() {
	util.ReadCommand(func(line string) error {
		fields := strings.Split(line, ";")
		if len(fields) != 2 {
			return nil
		}
		switch fields[0] {
		case "Chassis Service Tag":
			AddMeta("", nil, "svctag", fields[1], true)
		case "Chassis Model":
			AddMeta("", nil, "model", fields[1], true)
		}
		return nil
	}, "omreport", "chassis", "info", "-fmt", "ssv")
}
