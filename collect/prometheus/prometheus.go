package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ClusterState = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_status",
		Help: "Cluster status",
	})

	ClusterMemberMode = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_member_mode",
		Help: "Cluster member state",
	})

	ClusterPeersCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_peers_count",
		Help: "Amount of peers currently in cluster",
	})

	ClusterLastContact = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_last_contact",
		Help: "Last contact within cluster in unix timestamp",
	})

	ClusterLatestConfigurationIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_last_configuration_index",
		Help: "Last index of cluster configuration",
	})

	ClusterAppliedIndex = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_last_applied_index",
		Help: "Last cluster log applied index",
	})

	ClusterSnapshots = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_stapshots_count",
		Help: "Cluster snapshots were done",
	})

	ClusterSnapshotsErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_stapshots_errors",
		Help: "Cluster snapshots errors",
	})

	ClusterFailovers = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_failover_count",
		Help: "Amount of cluster failovers",
	})

	BosunChecksExecuted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bosun_checks_executed",
		Help: "Amount of checks that were executed",
	})
	BosunNotificationsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bosun_notifications_sent",
		Help: "Amount of checks that were executed",
	})
)

func Init() {
	prometheus.MustRegister(
		ClusterState,
		ClusterMemberMode,
		ClusterPeersCount,
		ClusterLastContact,
		ClusterLatestConfigurationIndex,
		ClusterAppliedIndex,
		ClusterSnapshots,
		ClusterSnapshotsErrors,
		ClusterFailovers,
		BosunChecksExecuted,
		BosunNotificationsSent,
	)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
