package metadata

import (
	"strings"

	"github.com/StackExchange/scollector/util"
)

func init() {
	metafuncs = append(metafuncs, collectMetadataOmreport)
}

func collectMetadataOmreport() {
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 {
			return
		}
		switch fields[0] {
		case "Chassis Service Tag":
			AddMeta("", nil, "svctag", fields[1], true)
		case "Chassis Model":
			AddMeta("", nil, "model", fields[1], true)
		}
	}, "omreport", "chassis", "info", "-fmt", "ssv")
}
