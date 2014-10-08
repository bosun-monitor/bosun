package collectors

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_netbackup_jobs})
}

//jobtype
// 0=backup, 1=archive, 2=restore, 3=verify, 4=duplicate, 5=import, 6=catalog backup, 7=vault, 8=label
// 9=erase, 10=tape request, 11=tape clean, 12=format tape, 13=physical inventory, 14=qualification
// 15=database recovery, 16=media contents, 17=image delete, 18=LiveUpdate

//state
// 0=queued, 1=active, 2=wait for retry, 3=done, 4=suspended, 5=incomplete

type NBJob struct {
	Jobid             string
	Jobtype           int
	State             int
	Status            string
	Class             string
	Schedule          string
	Client            string
	Server            string
	Started           time.Time
	Elapsed           string
	Ended             time.Time
	Stunit            string
	Try               string
	Operation         string
	Kbytes            int
	Files             int
	Pathlastwritten   string
	Percent           string
	Jobpid            string
	Owner             string
	Subtype           string
	Classtype         string
	Schedule_Type     string
	Priority          string
	Group             string
	Masterserver      string
	Retentionunits    string
	Retentionperiod   string
	Compression       string
	Kbyteslastwritten string
	Fileslastwritten  string
}

func NBUnmarshal(reader *csv.Reader, v interface{}) error {
	record, err := reader.Read()
	if err != nil {
		return err
	}
	if len(record) < 32 {
		return fmt.Errorf("Record to short, expected at least 32 fields, got %v", len(record))
	}
	s := reflect.ValueOf(v).Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		switch f.Type().String() {
		case "string":
			f.SetString(record[i])
		case "int":
			var ival int64
			if record[i] != "" {
				ival, err = strconv.ParseInt(record[i], 10, 64)
				if err != nil {
					return err
				}
			}
			f.SetInt(ival)
		case "time.Time":
			ival, err := strconv.ParseInt(record[i], 10, 0)
			if err != nil {
				return err
			}
			t := time.Unix(ival, 0)
			f.Set(reflect.ValueOf(t))
		default:
			return fmt.Errorf("Unsupported type: %v, ", f.Type().String())
		}
	}
	return nil
}

func c_netbackup_jobs() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	latest := make(map[string]NBJob)
	err := util.ReadCommand(func(line string) error {
		if len(line) < 32 {
			return nil
		}
		var r NBJob
		reader := csv.NewReader(strings.NewReader(line))
		err := NBUnmarshal(reader, &r)
		if err != nil {
			return err
		}
		if r.Jobtype != 0 {
			return nil
		}
		if r.State != 3 && r.State != 5 {
			return nil
		}
		key := r.Class + r.Schedule + r.Client
		if existing, ok := latest[key]; !ok {
			latest[key] = r
		} else {
			if r.Started.After(existing.Started) {
				latest[key] = r
			}
		}
		return nil
	}, `bpdbjobs`, "-report", "-all_columns")
	if err != nil {
		return md, err
	}
	now := time.Now()
	for _, r := range latest {
		tags := opentsdb.TagSet{"class": r.Class, "client": r.Client, "schedule": r.Schedule}
		Add(&md, "netbackup.backup.status", r.Status, tags, metadata.Gauge, "code", "")
		Add(&md, "netbackup.backup.duration", r.Elapsed, tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.attempt_age", now.Sub(r.Ended).Seconds(), tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.duration", r.Elapsed, tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.no_files", r.Files, tags, metadata.Gauge, metadata.Count, "")
		Add(&md, "netbackup.backup.kbytes", r.Kbytes, tags, metadata.Gauge, metadata.KBytes, "")

	}
	return md, err
}
