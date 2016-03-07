// +build darwin linux

package collectors

import (
	"strconv"
	"strings"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

type MetricSet map[string]string

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_nodestats_cfstats_linux})
}

func c_nodestats_cfstats_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var keyspace, table string
	util.ReadCommand(func(line string) error {
		fields := strings.Split(strings.TrimSpace(line), ": ")
		if len(fields) != 2 {
			return nil
		}
		if fields[0] == "Keyspace" {
			keyspace = fields[1]
			table = ""
			return nil
		}
		if fields[0] == "Table" {
			table = fields[1]
			return nil
		}

		tagset := make(opentsdb.TagSet)
		metricset := make(MetricSet)

		if table != "" {
			tagset["table"] = table
		}
		if keyspace != "" {
			tagset["keyspace"] = keyspace
		}

		metric := strings.Replace(fields[0], " ", "_", -1)
		metric = strings.Replace(metric, "(", "", -1)
		metric = strings.Replace(metric, ")", "", -1)
		metric = strings.Replace(metric, ",", "", -1)
		metric = strings.ToLower(metric)

		/*  This is to handle lines like "SSTables in each level: [31/4, 0, 0, 0, 0, 0, 0, 0]"
		sstables_in_each_level format (BNF):
		     <count>         ::= <integer>
		     <max_threshold> ::= <integer>
		     <exceeded>      ::= <count> "/" <max_threshold>
		     <level_item>    ::= <count>|<exceeded>
		     <list_item>     ::= <level_item> "," " " | <level_item>
		     <list>          ::= <list_item> | <list_item> <list>
		     <per_level>     ::= "[" <list> "]"
		*/
		if metric == "sstables_in_each_level" {
			fields[1] = strings.Replace(fields[1], "[", "", -1)
			fields[1] = strings.Replace(fields[1], "]", "", -1)
			per_level := strings.Split(fields[1], ", ")
			for index, count := range per_level {
				metricset["cassandra.tables.sstables_in_level_"+strconv.Itoa(index)] = strings.Split(count, "/")[0]
			}
			submitMetrics(&md, metricset, tagset)
			return nil
		}

		// every other value is simpler, and we only want the first word
		values := strings.Fields(fields[1])
		if _, err := strconv.ParseFloat(values[0], 64); err != nil || values[0] == "NaN" {
			return nil
		}

		metricset["cassandra.tables."+metric] = values[0]

		submitMetrics(&md, metricset, tagset)
		return nil
	}, "nodetool", "cfstats")
	return md, nil
}

func submitMetrics(md *opentsdb.MultiDataPoint, metricset MetricSet, tagset opentsdb.TagSet) {
	for m, v := range metricset {
		Add(md, m, v, tagset, metadata.Unknown, metadata.None, "")
	}
}
