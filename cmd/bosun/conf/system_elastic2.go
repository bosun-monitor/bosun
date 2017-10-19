// +build !esv5

package conf

import (
	"time"

	"bosun.org/cmd/bosun/expr"
	elastic "gopkg.in/olivere/elastic.v3"
)

// ParseESConfig return expr.ElasticHost
func parseESConfig(sc *SystemConf) expr.ElasticHosts {
	var options ESClientOptions
	esConf := expr.ElasticConfig{}
	store := make(map[string]expr.ElasticConfig)
	esHost := expr.ElasticHosts{}

	addClientOptions := func(item elastic.ClientOptionFunc) {
		esConf.ClientOptionFuncs = append(esConf.ClientOptionFuncs, item)
	}

	for hostPrefix, value := range sc.ElasticConf {
		options = value.ClientOptions

		if !options.Enabled {
			esConf.SimpleClient = value.SimpleClient
			esConf.Hosts = value.Hosts
			esConf.ClientOptionFuncs = esConf.ClientOptionFuncs[0:0]
			store[hostPrefix] = esConf
		} else {
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
			store[hostPrefix] = esConf
		}

		esHost.Hosts = store
	}

	return esHost
}

// ParseESConfig return expr.ElasticHost
func parseESAnnoteConfig(sc *SystemConf) expr.ElasticConfig {
	var options ESClientOptions
	esConf := expr.ElasticConfig{}

	addClientOptions := func(item elastic.ClientOptionFunc) {
		esConf.ClientOptionFuncs = append(esConf.ClientOptionFuncs, item)
	}

	options = sc.AnnotateConf.ClientOptions

	if !options.Enabled {
		esConf.SimpleClient = sc.AnnotateConf.SimpleClient
		esConf.Hosts = sc.AnnotateConf.Hosts
		return esConf
	}

	// SetURL
	addClientOptions(elastic.SetURL(sc.AnnotateConf.Hosts...))

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

	return esConf

}
