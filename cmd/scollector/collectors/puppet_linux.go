package collectors

import (
	"io/ioutil"
	"os"

	"github.com/bosun-monitor/scollector/metadata"
	"github.com/bosun-monitor/scollector/opentsdb"
	"gopkg.in/yaml.v1"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_linux, Enable: puppetEnable})
}

const (
	puppetPath       = "/var/lib/puppet/"
	puppetRunSummary = "/var/lib/puppet/state/last_run_summary.yaml"
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
	Time struct {
		Augeas           float64 `yaml:"augeas"`
		ConfigRetrieval  float64 `yaml:"config_retrieval"`
		Cron             float64 `yaml:"cron"`
		Exec             float64 `yaml:"exec"`
		File             float64 `yaml:"file"`
		Filebucket       float64 `yaml:"filebucket"`
		Group            float64 `yaml:"group"`
		IniSetting       float64 `yaml:"ini_setting"`
		LastRun          int64   `yaml:"last_run"`
		Package          float64 `yaml:"package"`
		Schedule         float64 `yaml:"schedule"`
		Service          float64 `yaml:"service"`
		SshAuthorizedKey float64 `yaml:"ssh_authorized_key"`
		Total            float64 `yaml:"total"`
		User             float64 `yaml:"user"`
		Yumrepo          float64 `yaml:"yumrepo"`
	} `yaml:"time"`
	Version struct {
		Config string `yaml:"config"`
		Puppet string `yaml:"puppet"`
	} `yaml:"version"`
}

func puppet_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	// See if puppet has been disabled (i.e. `puppet agent --disable 'Reason'`)
	disabled := 0
	if _, err := os.Stat(puppetDisabled); !os.IsNotExist(err) {
		disabled = 1
	}
	Add(&md, "puppet.disabled", disabled, nil, metadata.Unknown, metadata.None, "")
	// Gather stats from the run summary
	s, err := ioutil.ReadFile(puppetRunSummary)
	if err != nil {
		return nil, err
	}
	var m PRSummary
	if err = yaml.Unmarshal(s, &m); err != nil {
		return nil, err
	}
	//m.Version.Config appears to be the unix timestamp
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.Changed, opentsdb.TagSet{"resource": "changed"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.Failed, opentsdb.TagSet{"resource": "failed"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.FailedToRestart, opentsdb.TagSet{"resource": "failed_to_restart"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.OutOfSync, opentsdb.TagSet{"resource": "out_of_sync"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.Restarted, opentsdb.TagSet{"resource": "restarted"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.Scheduled, opentsdb.TagSet{"resource": "scheduled"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources", m.Time.LastRun, m.Resources.Changed, opentsdb.TagSet{"resource": "skipped"}, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.resources_total", m.Time.LastRun, m.Resources.Total, nil, metadata.Gauge, metadata.Count, "")
	AddTS(&md, "puppet.run.changes", m.Time.LastRun, m.Changes.Total, nil, metadata.Gauge, metadata.Count, "")
	return md, nil
}
