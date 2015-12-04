package collectors

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

func init() {
	os.Setenv("NLS_LANG", "AMERICAN_AMERICA.AL32UTF8")

	registerInit(func(c *conf.Conf) {
		for _, o := range c.Oracles {
			name := o.ClusterName
			for _, inst := range o.Instances {
				i := inst
				collectors = append(collectors, &IntervalCollector{
					F: func() (opentsdb.MultiDataPoint, error) {
						return c_oracle(name, i)
					},
					name: fmt.Sprintf("oracle-%s", name),
				})
			}
		}
	})
}

func c_oracle(name string, inst conf.OracleInstance) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	pr, pw := io.Pipe()
	errc := make(chan error, 1)

	p := &sqlplusParser{
		md:     &md,
		prefix: "oracle.",
		common: opentsdb.TagSet{
			"oracle_cluster": name,
		},
	}

	args := []string{"-S", inst.ConnectString}
	if role := inst.Role; role != "" {
		args = append(args, "as")
		args = append(args, role)
	}

	go func() {
		errc <- util.ReadCommandTimeout(10*time.Second, p.ParseAndAdd, pr, "sqlplus", args...)
	}()

	sqlplusWrite(pw)

	return md, <-errc
}

type sqlplusQueryRowParser struct {
	query string
	parse func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error
}

var (
	sqlplusParserFieldCountErr = errors.New("number of field doesn't match")
)

var sqlplusParsers = []sqlplusQueryRowParser{
	{
		"select name from v$database;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			// get database name and add it to common tag set
			common.Merge(opentsdb.TagSet{"oracle_database": row})
			return nil
		},
	},
	{
		"select instance_name from v$instance;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			// get instance name and add it to common tag set
			common.Merge(opentsdb.TagSet{"oracle_instance": row})
			return nil
		},
	},
	{
		"select METRIC_NAME || ',' || INTSIZE_CSEC || ',' || VALUE || ',' || METRIC_UNIT from v$sysmetric;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 4 {
				return sqlplusParserFieldCountErr
			}

			v, err := sqlplusValueConv(fields[2])
			if err != nil {
				return err
			}

			name := sqlplusMetricNameConv(fields[0])

			period, err := strconv.Atoi(fields[1])
			if err != nil {
				return err
			}
			period = (period/100 + 4) / 5 * 5 // handle rounding error

			name = prefix + name + "_" + strconv.Itoa(period) + "s"

			Add(md, name, v, common, metadata.Gauge, metadata.Unit(fields[3]), fields[0])
			return nil
		},
	},
	{
		"select NAME || ',' || VALUE from v$sysstat where NAME not like '%this session%';\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 2 {
				return sqlplusParserFieldCountErr
			}

			v, err := sqlplusValueConv(fields[1])
			if err != nil {
				return err
			}

			f0 := fields[0]
			name := sqlplusMetricNameConv(f0)

			rate := metadata.RateType(metadata.Counter)
			if f0 == "logons current" || strings.HasSuffix(f0, "cursors current") ||
				strings.HasPrefix(f0, "gc current") {
				rate = metadata.Gauge
			}

			Add(md, prefix+name, v, common, rate, metadata.None, f0)
			return nil
		},
	},
	{
		"select TABLESPACE_NAME || ',' || USED_PERCENT from dba_tablespace_usage_metrics;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 2 {
				return sqlplusParserFieldCountErr
			}

			v, err := sqlplusValueConv(fields[1])
			if err != nil {
				return err
			}

			ts := common.Copy().Merge(opentsdb.TagSet{"tablespace_name": fields[0]})
			Add(md, prefix+"tablespace_usage", v, ts, metadata.Gauge, metadata.Pct, "tablespace usage with autoextend and disk space be considered in")

			return nil
		},
	},
	{
		"select NAME || ',' || TYPE || ',' || TOTAL_MB || ',' || FREE_MB || ',' || USABLE_FILE_MB || ',' || OFFLINE_DISKS from v$asm_diskgroup_stat;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 6 {
				return sqlplusParserFieldCountErr
			}

			ts := common.Copy().Merge(opentsdb.TagSet{
				"diskgroup_name": fields[0],
				"diskgroup_type": fields[1],
			})

			v1, err1 := sqlplusValueConv(fields[2])
			v2, err2 := sqlplusValueConv(fields[3])
			v3, err3 := sqlplusValueConv(fields[4])
			v4, err4 := sqlplusValueConv(fields[5])

			for _, err := range []error{err1, err2, err3, err4} {
				if err != nil {
					return err
				}
			}

			rate := metadata.RateType(metadata.Gauge)

			Add(md, prefix+"asm_diskgroup.total_mb", v1, ts, rate, metadata.None, "asm disk group total space counted in megabytes")
			Add(md, prefix+"asm_diskgroup.free_mb", v2, ts, rate, metadata.None, "asm disk group free space counted in megabytes")
			Add(md, prefix+"asm_diskgroup.usable_file_mb", v3, ts, rate, metadata.None, "asm disk group usable space for storing files counted in megabytes")
			Add(md, prefix+"asm_diskgroup.offline_disks", v4, ts, rate, metadata.None, "asm disk group offline disk count")

			return nil
		},
	},
	{
		"select FAILGROUP || ',' || READS || ',' || WRITES || ',' || READ_ERRS || ',' || WRITE_ERRS || ',' || READ_TIME || ',' || WRITE_TIME || ',' || BYTES_READ || ',' || BYTES_WRITTEN from v$asm_disk_iostat;\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 9 {
				return sqlplusParserFieldCountErr
			}

			ts := common.Copy().Merge(opentsdb.TagSet{"failgroup": fields[0]})

			v1, err1 := sqlplusValueConv(fields[1])
			v2, err2 := sqlplusValueConv(fields[2])
			v3, err3 := sqlplusValueConv(fields[3])
			v4, err4 := sqlplusValueConv(fields[4])
			v5, err5 := sqlplusValueConv(fields[5])
			v6, err6 := sqlplusValueConv(fields[6])
			v7, err7 := sqlplusValueConv(fields[7])
			v8, err8 := sqlplusValueConv(fields[8])

			for _, err := range []error{err1, err2, err3, err4,
				err5, err6, err7, err8} {
				if err != nil {
					return err
				}
			}

			rate := metadata.RateType(metadata.Counter)

			Add(md, prefix+"asm_disk_iostat.reads", v1, ts, rate, metadata.None, "asm disk total reads")
			Add(md, prefix+"asm_disk_iostat.writes", v2, ts, rate, metadata.None, "asm disk total writes")
			Add(md, prefix+"asm_disk_iostat.read_errors", v3, ts, rate, metadata.Error, "asm disk total read errors")
			Add(md, prefix+"asm_disk_iostat.write_errors", v4, ts, rate, metadata.Error, "asm disk total write errors")
			Add(md, prefix+"asm_disk_iostat.read_time", v5, ts, rate, metadata.Second, "asm disk total read time in second")
			Add(md, prefix+"asm_disk_iostat.write_time", v6, ts, rate, metadata.Second, "asm disk total write time in second")
			Add(md, prefix+"asm_disk_iostat.bytes_read", v7, ts, rate, metadata.Bytes, "asm disk total bytes read")
			Add(md, prefix+"asm_disk_iostat.bytes_written", v8, ts, rate, metadata.Bytes, "asm disk total bytes written")

			return nil
		},
	},
	{
		"select b.WAIT_CLASS || ',' || a.AVERAGE_WAITER_COUNT || ',' || a.DBTIME_IN_WAIT || ',' || b.TOTAL_WAITS || ',' || b.TIME_WAITED || ',' || b.TOTAL_WAITS_FG || ',' || b.TIME_WAITED_FG from v$waitclassmetric a, v$system_wait_class b where a.WAIT_CLASS_ID = b.WAIT_CLASS_ID and WAIT_CLASS <> 'Idle';\n",
		func(row string, md *opentsdb.MultiDataPoint, prefix string, common opentsdb.TagSet) error {
			fields := strings.Split(row, ",")
			if len(fields) != 7 {
				return sqlplusParserFieldCountErr
			}

			ts := common.Copy().Merge(opentsdb.TagSet{"wait_class": fields[0]})

			v1, err1 := sqlplusValueConv(fields[1])
			v2, err2 := sqlplusValueConv(fields[2])
			v3, err3 := sqlplusValueConv(fields[3])
			v4, err4 := sqlplusValueConv(fields[4])
			v5, err5 := sqlplusValueConv(fields[5])
			v6, err6 := sqlplusValueConv(fields[6])

			for _, err := range []error{err1, err2, err3, err4, err5, err6} {
				if err != nil {
					return err
				}
			}

			Add(md, prefix+"wait_class.avg_waiter_1m", v1, ts, metadata.Gauge, metadata.None, "average waiter count for one minute")
			Add(md, prefix+"wait_class.avg_dbtime_wait_1m", v2, ts, metadata.Gauge, metadata.Pct, "average database time in wait for one minute")
			Add(md, prefix+"wait_class.total_waits", v3, ts, metadata.Counter, metadata.None, "total waits")
			Add(md, prefix+"wait_class.total_time_waited", v4, ts, metadata.Counter, metadata.Second, "total time waited")
			Add(md, prefix+"wait_class.total_foreground_waits", v5, ts, metadata.Counter, metadata.None, "total foreground waits")
			Add(md, prefix+"wait_class.total_time_foreground_waited", v6, ts, metadata.Counter, metadata.Second, "total time foreground waited")

			return nil
		},
	},
}

type sqlplusParser struct {
	parsedQuery int

	md     *opentsdb.MultiDataPoint
	prefix string
	common opentsdb.TagSet
}

func (p *sqlplusParser) ParseAndAdd(line string) error {
	parsed, n := p.parsedQuery, len(sqlplusParsers)

	// query result separator is blank line
	if line == "" {
		return nil
	}

	// handle feed, end of one query
	if line == "no rows selected" || strings.HasSuffix(line, " rows selected.") ||
		strings.HasSuffix(line, " row selected.") {
		p.parsedQuery++
		return nil
	}

	// finished all queries
	if parsed == n {
		return nil
	}

	// process actual queries
	if err := sqlplusParsers[parsed].parse(line, p.md, p.prefix, p.common); err != nil {
		slog.Errorln("oracle sqlplus parser error:", err)
	}
	return nil
}

func sqlplusFormatOutput(w io.Writer) (err error) {
	_, err = io.WriteString(w, "set linesize 32767;\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set pagesize 32767;\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set head off;\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set feed on;\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set colsep \",\";\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set trimspool on;\n")
	if err != nil {
		return
	}

	_, err = io.WriteString(w, "set trimout on;\n")
	return
}

func sqlplusExit(w io.Writer) (err error) {
	_, err = io.WriteString(w, "exit;\n")
	return
}

func sqlplusWrite(pw *io.PipeWriter) {
	var err error
	defer func() {
		pw.CloseWithError(err)
	}()

	err = sqlplusFormatOutput(pw)
	if err != nil {
		return
	}

	for _, p := range sqlplusParsers {
		_, err = io.WriteString(pw, p.query)
		if err != nil {
			return
		}
	}

	err = sqlplusExit(pw)
}

func sqlplusValueConv(value string) (interface{}, error) {
	if strings.Contains(value, ".") {
		// opentsdb only accept single precision floating point number
		return strconv.ParseFloat(value, 32)
	}
	return strconv.ParseInt(value, 10, 64)
}

func sqlplusMetricNameConv(name string) string {
	name = strings.Replace(name, "<", "below_", -1)
	name = strings.Replace(name, ">=", "above_", -1)
	name = strings.Replace(name, "(", "", -1)
	name = strings.Replace(name, ")", "", -1)
	name = strings.Replace(name, "%", "ratio", -1)

	// ignore this error
	name, _ = opentsdb.Replace(strings.ToLower(name), "_")
	return name
}
