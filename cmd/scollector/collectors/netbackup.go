package collectors

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: cNetbackupJobs})
	collectors = append(collectors, &IntervalCollector{F: cNetbackupFrequency})
}

//jobtype
// 0=backup, 1=archive, 2=restore, 3=verify, 4=duplicate, 5=import, 6=catalog backup, 7=vault, 8=label
// 9=erase, 10=tape request, 11=tape clean, 12=format tape, 13=physical inventory, 14=qualification
// 15=database recovery, 16=media contents, 17=image delete, 18=LiveUpdate

//state
// 0=queued, 1=active, 2=wait for retry, 3=done, 4=suspended, 5=incomplete

//NOTE!!
// This depends on the retention of the job log being greater than the jobs,
// else things are going to go unknown. See
// http://www.symantec.com/connect/forums/netbackup-75-activity-monitor-job-logs
// In my case I created the two registry entries mentioned in that link
// (KEEP_JOB_HOURS) and (KEEP_JOBS_SUCCESSFUL_HOURS). I also changed the
// rentention under "Clean-up" under the master server properties via the Java
// Admin Console. One of those seems to have worked. This *is* netbackup, so I
// wish you the best of luck ;-).

type nbJob struct {
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
	ScheduleType      string
	Priority          string
	Group             string
	Masterserver      string
	Retentionunits    string
	Retentionperiod   string
	Compression       string
	Kbyteslastwritten string
	Fileslastwritten  string
}

var timeType = reflect.TypeOf(time.Time{})

func nbUnmarhsall(reader *csv.Reader, v interface{}) error {
	record, err := reader.Read()
	if err != nil {
		return err
	}
	if len(record) < 32 {
		return fmt.Errorf("record too short, expected at least 32 fields, got %v", len(record))
	}
	s := reflect.ValueOf(v).Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString(record[i])
		case reflect.Int:
			var ival int64
			if record[i] == "" {
				continue
			}
			ival, err = strconv.ParseInt(record[i], 10, 64)
			if err != nil {
				return err
			}
			f.SetInt(ival)
		case reflect.Struct:
			switch f.Type() {
			case timeType:
				ival, err := strconv.ParseInt(record[i], 10, 64)
				if err != nil {
					return err
				}
				t := time.Unix(ival, 0)
				f.Set(reflect.ValueOf(t))
			default:
				return fmt.Errorf("unsupported type: %s", f.Type())
			}
		default:
			return fmt.Errorf("unsupported type: %s", f.Type())
		}
	}
	return nil
}

func cNetbackupJobs() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	latest := make(map[string]nbJob)
	if err := util.ReadCommand(func(line string) error {
		if len(line) < 32 {
			return nil
		}
		var r nbJob
		reader := csv.NewReader(strings.NewReader(line))
		if err := nbUnmarhsall(reader, &r); err != nil {
			return err
		}
		if !(r.Jobtype == 0 || r.Jobtype == 6) {
			return nil
		}
		if r.State != 3 && r.State != 5 {
			return nil
		}
		key := r.Class + r.Schedule + r.Client
		if existing, ok := latest[key]; !ok {
			latest[key] = r
		} else if r.Started.After(existing.Started) {
			latest[key] = r
		}
		return nil
	}, "bpdbjobs", "-report", "-all_columns"); err == util.ErrPath {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, r := range latest {
		tags := opentsdb.TagSet{"class": r.Class, "client": r.Client, "schedule": r.Schedule}
		Add(&md, "netbackup.backup.status", r.Status, tags, metadata.Gauge, metadata.StatusCode, "")
		Add(&md, "netbackup.backup.duration", r.Elapsed, tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.attempt_age", now.Sub(r.Ended).Seconds(), tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.duration", r.Elapsed, tags, metadata.Gauge, metadata.Second, "")
		Add(&md, "netbackup.backup.no_files", r.Files, tags, metadata.Gauge, metadata.Count, "")
		Add(&md, "netbackup.backup.kbytes", r.Kbytes, tags, metadata.Gauge, metadata.KBytes, "")
	}
	return md, nil
}

func cNetbackupFrequency() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var class, schedule string
	var clients []string
	if err := util.ReadCommand(func(line string) error {
		if strings.HasPrefix(line, "Policy Name:") {
			clients = nil
			f := strings.Fields(line)
			if len(f) == 3 {
				class = f[2]
				return nil
			}
			return fmt.Errorf("error parsing policy: %v", line)
		}
		if strings.HasPrefix(line, "Client/HW/OS/Pri/DMI/CIT:") {
			f := strings.Fields(line)
			if len(f) == 9 {
				clients = append(clients, f[1])
				return nil
			}
			return fmt.Errorf("error parsing client")
		}
		if strings.HasPrefix(line, "Schedule:") {
			f := strings.Fields(line)
			if len(f) > 1 {
				schedule = f[1]
				return nil
			}
			return fmt.Errorf("error parsing client: %v", line)
		}
		if strings.HasPrefix(strings.TrimSpace(line), "Frequency:") {
			f := strings.Fields(line)
			if len(f) == 5 {
				freq := strings.TrimLeft(f[3], "(")
				for _, client := range clients {
					tags := opentsdb.TagSet{"class": class, "client": client, "schedule": schedule}
					Add(&md, "netbackup.backup.frequency", freq, tags, metadata.Gauge, metadata.Second, "")
				}
				return nil
			}
			return fmt.Errorf("error parsing frequency: %v", line)
		}
		return nil
	}, "bppllist", "-L", "-allpolicies"); err == util.ErrPath {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return md, nil
}
