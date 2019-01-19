package tsdbs

import (
	ainsightsmgmt "github.com/Azure/azure-sdk-for-go/services/appinsights/mgmt/2015-05-01/insights"
	ainsights "github.com/Azure/azure-sdk-for-go/services/appinsights/v1/insights"
	"github.com/Azure/azure-sdk-for-go/services/preview/monitor/mgmt/2018-03-01/insights"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// AzureMonitorClientCollection is a collection of Azure SDK clients since
// the SDK provides different clients to access different sorts of resources
type AzureMonitorClientCollection struct {
	MetricsClient           insights.MetricsClient
	MetricDefinitionsClient insights.MetricDefinitionsClient
	ResourcesClient         resources.Client
	AIComponentsClient      ainsightsmgmt.ComponentsClient
	AIMetricsClient         ainsights.MetricsClient
	Concurrency             int
	TenantId                string
}

// AzureMonitorClients is map of all the AzureMonitorClientCollections that
// have been configured. This is so multiple subscription/tenant/clients
// can be queries from the same Bosun instance using the prefix syntax
type AzureMonitorClients map[string]AzureMonitorClientCollection

// PromClients is a collection of Prometheus API v1 client APIs (connections)
type PromClients map[string]promv1.API

// ElasticHosts is an array of Logstash hosts and exists as a type for something to attach
// methods to.  The elasticsearch library will use the listed to hosts to discover all
// of the hosts in the config
// type ElasticHosts []string
type ElasticHosts struct {
	Hosts map[string]ElasticConfig
}

type ElasticConfig struct {
	Hosts             []string
	Version           string
	SimpleClient      bool
	ClientOptionFuncs interface{}
}
