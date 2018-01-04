package collectors

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"gopkg.in/yaml.v1"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_linux, Enable: puppetEnable})
}

const (
	puppetPath       = "/var/lib/puppet/"
	puppetRunSummary = "/var/lib/puppet/state/last_run_summary.yaml"
	puppetRunReport  = "/var/lib/puppet/state/last_run_report.yaml"
	puppetDisabled   = "/var/lib/puppet/state/agent_disabled.lock"
)

func puppetEnable() bool {
	_, err := os.Stat(puppetPath)
	return err == nil
}

type PRSummary struct {
	Changes struct {
		Total float64 `yaml:"total"`
	} `yaml:"changes"`
	Events struct {
		Failure float64 `yaml:"failure"`
		Success float64 `yaml:"success"`
		Total   float64 `yaml:"total"`
	} `yaml:"events"`
	Resources struct {
		Changed         float64 `yaml:"changed"`
		Failed          float64 `yaml:"failed"`
		FailedToRestart float64 `yaml:"failed_to_restart"`
		OutOfSync       float64 `yaml:"out_of_sync"`
		Restarted       float64 `yaml:"restarted"`
		Scheduled       float64 `yaml:"scheduled"`
		Skipped         float64 `yaml:"skipped"`
		Total           float64 `yaml:"total"`
	} `yaml:"resources"`
	Time    map[string]string `yaml:"time"`
	Version struct {
		Config string `yaml:"config"`
		Puppet string `yaml:"puppet"`
	} `yaml:"version"`
}

type PRReport struct {
	Status string `yaml:"status"`
	Time   string `yaml:"time"` // 2006-01-02 15:04:05.999999 -07:00
}

func puppet_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	// See if puppet has been disabled (i.e. `puppet agent --disable 'Reason'`)
	var disabled, noReason int
	if v, err := ioutil.ReadFile(puppetDisabled); err == nil {
		disabled = 1
		d := struct {
			Disabled string `json:"disabled_message"`
		}{}
		if err := json.Unmarshal(v, &d); err == nil && d.Disabled != "" {
			if d.Disabled == "reason not specified" {
				noReason = 1
			}
			metadata.AddMeta("", nil, "puppet.disabled_reason", d.Disabled, true)
		}
	}
	Add(&md, "puppet.disabled", disabled, nil, metadata.Gauge, metadata.Count, "")
	Add(&md, "puppet.disabled_no_reason", noReason, nil, metadata.Gauge, metadata.Count, "")
	// Gather stats from the run summary
	s, err := ioutil.ReadFile(puppetRunSummary)
	if err != nil {
		return nil, err
	}
	var m PRSummary
	if err = yaml.Unmarshal(s, &m); err != nil {
		return nil, err
	}
	last_run, err := strconv.ParseInt(m.Time["last_run"], 10, 64)
	seconds_since_run := time.Now().Unix() - last_run
	//m.Version.Config appears to be the unix timestamp
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.Changed, opentsdb.TagSet{"resource": "changed"}, metadata.Gauge, metadata.Count, descPuppetChanged)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.Failed, opentsdb.TagSet{"resource": "failed"}, metadata.Gauge, metadata.Count, descPuppetFailed)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.FailedToRestart, opentsdb.TagSet{"resource": "failed_to_restart"}, metadata.Gauge, metadata.Count, descPuppetFailedToRestart)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.OutOfSync, opentsdb.TagSet{"resource": "out_of_sync"}, metadata.Gauge, metadata.Count, descPuppetOutOfSync)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.Restarted, opentsdb.TagSet{"resource": "restarted"}, metadata.Gauge, metadata.Count, descPuppetRestarted)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.Scheduled, opentsdb.TagSet{"resource": "scheduled"}, metadata.Gauge, metadata.Count, descPuppetScheduled)
	AddTS(&md, "puppet.run.resources", last_run, m.Resources.Skipped, opentsdb.TagSet{"resource": "skipped"}, metadata.Gauge, metadata.Count, descPuppetSkipped)
	AddTS(&md, "puppet.run.resources_total", last_run, m.Resources.Total, nil, metadata.Gauge, metadata.Count, descPuppetTotalResources)
	AddTS(&md, "puppet.run.changes", last_run, m.Changes.Total, nil, metadata.Gauge, metadata.Count, descPuppetTotalChanges)
	Add(&md, "puppet.last_run", seconds_since_run, nil, metadata.Gauge, metadata.Second, descPuppetLastRun)
	for k, v := range m.Time {
		metric, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return md, fmt.Errorf("Error parsing time: %s", err)
		}
		if k == "total" {
			AddTS(&md, "puppet.run_duration_total", last_run, metric, nil, metadata.Gauge, metadata.Second, descPuppetTotalTime)
		} else if k != "last_run" {
			AddTS(&md, "puppet.run_duration", last_run, metric, opentsdb.TagSet{"time": k}, metadata.Gauge, metadata.Second, descPuppetModuleTime)
		}
	}

	// Not all hosts will use puppet run reports
	if _, err := os.Stat(puppetRunReport); err == nil {
		f, err := ioutil.ReadFile(puppetRunReport)
		if err != nil {
			return md, err
		}

		var report PRReport
		if err = yaml.Unmarshal(f, &report); err != nil {
			return md, err
		}

		t, err := time.Parse("2006-01-02 15:04:05.999999 -07:00", report.Time)
		if err != nil {
			// Puppet 5 changed the time format
			t, err = time.Parse("2006-01-02T15:04:05.999999-07:00", report.Time)
		}
		if err != nil {
			return md, fmt.Errorf("Error parsing report time: %s", err)
		}
		// As listed at https://docs.puppetlabs.com/puppet/latest/reference/format_report.html
		var statusCode = map[string]int{
			"changed":   0,
			"unchanged": 1,
			"failed":    2,
		}
		if status, ok := statusCode[report.Status]; ok {
			AddTS(&md, "puppet.run.status", t.Unix(), status, nil, metadata.Gauge, metadata.StatusCode, descPuppetRunStatus)
		} else {
			return md, fmt.Errorf("Unknown status in %s: %s", puppetRunReport, report.Status)
		}
	}
	return md, nil
}

const (
	descPuppetChanged         = "Number of resources for which changes were applied."
	descPuppetFailed          = "Number of resources which caused an error during evaluation."
	descPuppetFailedToRestart = "Number of service resources which failed to restart."
	descPuppetOutOfSync       = "Number of resources which should have been changed if catalog was applied."
	descPuppetRestarted       = "Number of service resources which were restarted."
	descPuppetScheduled       = "Number of service resources which were scheduled for restart."
	descPuppetSkipped         = "Number of resources which puppet opted to not apply changes to."
	descPuppetTotalResources  = "Total number of resources."
	descPuppetTotalChanges    = "Total number of changes."
	descPuppetTotalTime       = "Total time which puppet took to run."
	descPuppetModuleTime      = "Time which this tagged module took to run."
	descPuppetLastRun         = "Number of seconds since puppet run last ran."
	descPuppetRunStatus       = "0: changed, 1: unchanged, 2: failed"
)
