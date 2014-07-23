package collectors

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
	"gopkg.in/yaml.v1"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: puppet_linux, init: puppetInit})
}

var (
	puppetEnable bool
	puppetLock   sync.Mutex
)

const (
	puppetPath       = "/var/lib/puppet/"
	puppetRunSummary = "/var/lib/puppet/state/last_run_summary.yaml"
	puppetDisabled   = "/var/lib/puppet/state/agent_disabled.lock"
)

func puppetEnabled() (b bool) {
	puppetLock.Lock()
	b = puppetEnable
	puppetLock.Unlock()
	return
}

func puppetInit() {
	update := func() {
		_, err := os.Stat(puppetPath)
		puppetLock.Lock()
		puppetEnable = err == nil
		puppetLock.Unlock()
	}
	update()
	go func() {
		for _ = range time.Tick(time.Minute * 5) {
			update()
		}
	}()
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
		LastRun          float64 `yaml:"last_run"`
		Package          float64 `yaml:"package"`
		Schedule         float64 `yaml:"schedule"`
		Service          float64 `yaml:"service"`
		SshAuthorizedKey float64 `yaml:"ssh_authorized_key"`
		Total            float64 `yaml:"total"`
		User             float64 `yaml:"user"`
		Yumrepo          float64 `yaml:"yumrepo"`
	} `yaml:"time"`
	Version struct {
		Config int64  `yaml:"config"`
		Puppet string `yaml:"puppet"`
	} `yaml:"version"`
}

func puppet_linux() opentsdb.MultiDataPoint {
	if !puppetEnabled() {
		return nil
	}
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
		slog.Errorln(err)
		return nil
	}
	var m PRSummary
	if err = yaml.Unmarshal(s, &m); err != nil {
		slog.Errorln(err)
		return nil
	}
	fmt.Println(m.Resources.Changed)
	//m.Version.Config appears to be the unix timestamp
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.Changed, opentsdb.TagSet{"resource": "changed"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.Failed, opentsdb.TagSet{"resource": "failed"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.FailedToRestart, opentsdb.TagSet{"resource": "failed_to_restart"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.OutOfSync, opentsdb.TagSet{"resource": "out_of_sync"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.Restarted, opentsdb.TagSet{"resource": "restarted"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.Scheduled, opentsdb.TagSet{"resource": "scheduled"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources", m.Version.Config, m.Resources.Changed, opentsdb.TagSet{"resource": "skipped"}, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.resources_total", m.Version.Config, m.Resources.Total, nil, metadata.Unknown, metadata.None, "")
	AddTS(&md, "puppet.run.changes", m.Version.Config, m.Changes.Total, nil, metadata.Unknown, metadata.None, "")
	return md
}
