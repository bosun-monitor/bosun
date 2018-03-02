package collectors

const (
	puppetPath       = "/var/lib/puppet/"
	puppetRunSummary = "/var/lib/puppet/state/last_run_summary.yaml"
	puppetRunReport  = "/var/lib/puppet/state/last_run_report.yaml"
	puppetDisabled   = "/var/lib/puppet/state/agent_disabled.lock"
)
