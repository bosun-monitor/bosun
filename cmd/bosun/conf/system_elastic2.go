package conf

import (
	"time"

	"bosun.org/cmd/bosun/expr"
	elastic "gopkg.in/olivere/elastic.v3"
)

// ParseESConfig return expr.ElasticHost
func parseESConfig2(value ElasticConf) expr.ElasticConfig {
	var esConf expr.ElasticConfig
	var options ESClientOptions
	var opts []elastic.ClientOptionFunc

	// method to append clinet options
	addClientOptions := func(item elastic.ClientOptionFunc) {
		opts = append(opts, item)
	}

	options = value.ClientOptions
	if !options.Enabled {
		esConf.SimpleClient = value.SimpleClient
		esConf.Hosts = value.Hosts
		esConf.ClientOptionFuncs = opts[0:0]
		return esConf
	}

	// SetURL
	addClientOptions(elastic.SetURL(value.Hosts...))

	if options.BasicAuthUsername != "" && options.BasicAuthPassword != "" {
		addClientOptions(elastic.SetBasicAuth(options.BasicAuthUsername, options.BasicAuthPassword))
	}

	if options.Scheme == "https" {
		addClientOptions(elastic.SetScheme(options.Scheme))
	}

	// Default Enable
	addClientOptions(elastic.SetSniff(options.SnifferEnabled))

	if options.SnifferTimeoutStartup > 5 {
		options.SnifferTimeoutStartup = options.SnifferTimeoutStartup * time.Second
		addClientOptions(elastic.SetSnifferTimeoutStartup(options.SnifferTimeoutStartup))
	}

	if options.SnifferTimeout > 2 {
		options.SnifferTimeout = options.SnifferTimeout * time.Second
		addClientOptions(elastic.SetSnifferTimeout(options.SnifferTimeout))
	}

	if options.SnifferInterval > 15 {
		options.SnifferInterval = options.SnifferInterval * time.Minute
		addClientOptions(elastic.SetSnifferInterval(options.SnifferTimeout))
	}

	//Default Enable
	addClientOptions(elastic.SetHealthcheck(options.HealthcheckEnabled))

	if options.HealthcheckTimeoutStartup > 5 {
		options.HealthcheckTimeoutStartup = options.HealthcheckTimeoutStartup * time.Second
		addClientOptions(elastic.SetHealthcheckTimeoutStartup(options.HealthcheckTimeoutStartup))
	}

	if options.HealthcheckTimeout > 1 {
		options.HealthcheckTimeout = options.HealthcheckTimeout * time.Second
		addClientOptions(elastic.SetHealthcheckTimeout(options.HealthcheckTimeout))
	}

	if options.HealthcheckInterval > 60 {
		options.HealthcheckInterval = options.HealthcheckInterval * time.Second
		addClientOptions(elastic.SetHealthcheckInterval(options.HealthcheckInterval))
	}

	if options.MaxRetries > 0 {
		addClientOptions(elastic.SetMaxRetries(options.MaxRetries))
	}
	esConf.Hosts = esConf.Hosts[0:0]
	esConf.SimpleClient = false
	esConf.ClientOptionFuncs = opts

	return esConf
}
