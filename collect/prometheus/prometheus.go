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

	ClusterApplyLogErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_apply_log_errors",
		Help: "Cluster apply log errors",
	})

	ClusterFailovers = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cluster_failover_count",
		Help: "Amount of cluster failovers",
	})

	BosunChecksExecuted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bosun_checks_executed",
		Help: "Amount of checks that were executed",
	})
	BosunNotificationSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bosun_notification_sent",
			Help: "Amount of notifications that were sent",
		},
		[]string{"transport"},
	)
	BosunNotificationError = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bosun_notification_errors",
			Help: "Amount of notifications that weren't sent because error",
		},
		[]string{"transport"},
	)
	BosunNotificationLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "bosun_notifications_latency",
			Help:       "Latencies for notifications per channel (sec)",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"transport"},
	)
	BosunIncidentChanged = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bosun_incidents_changed",
			Help: "Amount of incidents that were opened by status",
		},
		[]string{"state"},
	)
	BosunDatabaseGetConnectorLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "bosun_database_get_connector_latency",
			Help:       "Latencies for database queries (sec)",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"query"},
	)
	BosunDatabaseQueriesLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "bosun_database_query_latency",
			Help:       "Latencies for database queries (sec)",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"caller", "query"},
	)
	BosunDatabaseQueryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bosun_database_query_errors",
			Help: "Amount of queries with error",
		},
		[]string{"caller", "query"},
	)
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
		ClusterApplyLogErrors,
		ClusterFailovers,
		BosunChecksExecuted,
		BosunNotificationSent,
		BosunNotificationError,
		BosunNotificationLatency,
		BosunIncidentChanged,
		BosunDatabaseQueriesLatency,
		BosunDatabaseGetConnectorLatency,
		BosunDatabaseQueryErrors,
	)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}
