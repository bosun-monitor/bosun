// +build darwin linux

package collectors

import (
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_nodestats_cfstats_linux})
}

func c_nodestats_cfstats_linux() opentsdb.MultiDataPoint {
	var md opentsdb.MultiDataPoint
	var keyspace, table string
	util.ReadCommand(func(line string) {
		fields := strings.Split(strings.TrimSpace(line), ": ")
		if len(fields) != 2 {
			return
		}
		if fields[0] == "Keyspace" {
			keyspace = fields[1]
			table = ""
			return
		}
		if fields[0] == "Table" {
			table = fields[1]
			return
		}
		metric := strings.Replace(fields[0], " ", "_", -1)
		metric = strings.Replace(metric, "(", "", -1)
		metric = strings.Replace(metric, ")", "", -1)
		metric = strings.Replace(metric, ",", "", -1)
		metric = strings.ToLower(metric)
		values := strings.Fields(fields[1])
		if values[0] == "NaN" {
			return
		}
		value := values[0]
		if table == "" {
			Add(&md, "cassandra.tables."+metric, value, opentsdb.TagSet{"keyspace": keyspace}, metadata.Unknown, metadata.None, "")
		} else {
			Add(&md, "cassandra.tables."+metric, value, opentsdb.TagSet{"keyspace": keyspace, "table": table}, metadata.Unknown, metadata.None, "")
		}
	}, "nodetool", "cfstats")
	return md
}
