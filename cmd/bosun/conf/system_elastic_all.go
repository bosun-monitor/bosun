package conf

import (
	"fmt"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/slog"
)

// ParseESConfig return expr.ElasticHost
func parseESConfig(sc *SystemConf) expr.ElasticHosts {
	store := make(map[string]expr.ElasticConfig)
	for hostPrefix, value := range sc.ElasticConf {
		// build es config per cluster
		var cfg expr.ElasticConfig
		switch expr.ESVersion(value.Version) {
		case expr.ESV2:
			cfg = parseESConfig2(value)
		case expr.ESV5:
			cfg = parseESConfig5(value)
		case expr.ESV6:
			cfg = parseESConfig6(value)
		case "":
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: Version is required a field (supported values for Version are: "v2", "v5", and "v6")`, hostPrefix))
		default:
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: invalid elastic version: %s (supported versions are: "v2", "v5", and "v6")`, hostPrefix, value.Version))
		}

		cfg.Version = expr.ESVersion(value.Version)
		store[hostPrefix] = cfg
	}

	return expr.ElasticHosts{Hosts: store}
}

// ParseESConfig return expr.ElasticHost
func parseESAnnoteConfig(sc *SystemConf) expr.ElasticConfig {
	var cfg expr.ElasticConfig
	if len(sc.AnnotateConf.Hosts) == 0 {
		return cfg
	}
	switch expr.ESVersion(sc.AnnotateConf.Version) {
	case expr.ESV2:
		cfg = parseESConfig2(ElasticConf(sc.AnnotateConf))
	case expr.ESV5:
		cfg = parseESConfig5(ElasticConf(sc.AnnotateConf))
	case expr.ESV6:
		cfg = parseESConfig6(ElasticConf(sc.AnnotateConf))
	case "":
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: Version is required a field (supported values for Version are: "v2", "v5", and "v6")`))
	default:
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: invalid elastic version: %s (supported versions are: "v2", "v5", and "v6")`, sc.AnnotateConf.Version))
	}

	cfg.Version = expr.ESVersion(sc.AnnotateConf.Version)
	return cfg
}
