package collectors

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

const eximExiqsumm = "/usr/sbin/exiqsumm"

func init() {
	collectors = append(collectors, &IntervalCollector{
		F:        c_exim_mailq,
		Interval: time.Minute,
		Enable:   EnableExecutable(eximExiqsumm),
	})
}

func c_exim_mailq() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	mailq, err := util.Command(time.Minute, nil, "/usr/bin/mailq")
	if err != nil {
		return nil, err
	}
	util.ReadCommandTimeout(time.Minute, func(line string) error {
		f := strings.Fields(line)
		if len(f) == 5 && f[4] == "TOTAL" {
			Add(&md, "exim.mailq_count", f[0], nil, metadata.Gauge, metadata.EMail, "The number of emails in exim's mail queue.")
			var multi int64 = 1
			size, err := strconv.ParseInt(f[1], 10, 64)
			if err != nil && len(f[1]) > 3 {
				unit := f[1][len(f[1])-2:]
				switch unit {
				case "KB":
					multi = 1024
				case "MB":
					multi = 1024 * 1024
				default:
					return fmt.Errorf("error parsing size unit of exim's mail queue")
				}
				size, err = strconv.ParseInt(f[1][:len(f[1])-2], 10, 64)
				if err != nil {
					return fmt.Errorf("error parsing exim size field")
				}
			}
			Add(&md, "exim.mailq_size", size*multi, nil, metadata.Gauge, metadata.Bytes, descEximMailQSize)
			oldest, err := opentsdb.ParseDuration(f[2])
			if err != nil {
				return err
			}
			Add(&md, "exim.mailq_oldest", oldest.Seconds(), nil, metadata.Gauge, metadata.Second, descEximMailQOldest)
			newest, err := opentsdb.ParseDuration(f[3])
			if err != nil {
				return err
			}
			Add(&md, "exim.mailq_newest", newest.Seconds(), nil, metadata.Gauge, metadata.Second, descEximMailQNewest)
		}
		return nil
	}, mailq, eximExiqsumm)
	return md, nil
}

const (
	descEximMailQSize   = "The size of all emails in exim's mail queue."
	descEximMailQOldest = "Age of the oldest email in exim's mail queue."
	descEximMailQNewest = "Age of the newest email in exim's mail queue."
)
