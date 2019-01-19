package conf

import (
	"fmt"

	"bosun.org/cmd/bosun/expr/tsdbs"
	esExpr "bosun.org/cmd/bosun/expr/tsdbs/elastic"
	"bosun.org/slog"
)

// ParseESConfig return expr.ElasticHost
func parseESConfig(sc *SystemConf) tsdbs.ElasticHosts {
	store := make(map[string]tsdbs.ElasticConfig)
	for hostPrefix, value := range sc.ElasticConf {
		// build es config per cluster
		var cfg tsdbs.ElasticConfig
		switch esExpr.ESVersion(value.Version) {
		case esExpr.ESV2:
			cfg = parseESConfig2(value)
		case esExpr.ESV5:
			cfg = parseESConfig5(value)
		case esExpr.ESV6:
			cfg = parseESConfig6(value)
		case "":
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: Version is required a field (supported values for Version are: "v2", "v5", and "v6")`, hostPrefix))
		default:
			slog.Fatal(fmt.Errorf(`conf: [ElasticConf.%s]: invalid elastic version: %s (supported versions are: "v2", "v5", and "v6")`, hostPrefix, value.Version))
		}

		cfg.Version = string(esExpr.ESVersion(value.Version))
		store[hostPrefix] = cfg
	}

	return tsdbs.ElasticHosts{Hosts: store}
}

// ParseESConfig return tsdbs.ElasticHost
func parseESAnnoteConfig(sc *SystemConf) tsdbs.ElasticConfig {
	var cfg tsdbs.ElasticConfig
	if len(sc.AnnotateConf.Hosts) == 0 {
		return cfg
	}
	switch esExpr.ESVersion(sc.AnnotateConf.Version) {
	case esExpr.ESV2:
		cfg = parseESConfig2(ElasticConf(sc.AnnotateConf))
	case esExpr.ESV5:
		cfg = parseESConfig5(ElasticConf(sc.AnnotateConf))
	case esExpr.ESV6:
		cfg = parseESConfig6(ElasticConf(sc.AnnotateConf))
	case "":
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: Version is required a field (supported values for Version are: "v2", "v5", and "v6")`))
	default:
		slog.Fatal(fmt.Errorf(`conf: [AnnotateConf]: invalid elastic version: %s (supported versions are: "v2", "v5", and "v6")`, sc.AnnotateConf.Version))
	}

	cfg.Version = string(esExpr.ESVersion(sc.AnnotateConf.Version))
	return cfg
}
