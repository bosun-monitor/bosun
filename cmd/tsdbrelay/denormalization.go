package main

import (
	"fmt"
	"log"
	"strings"

	"bosun.org/opentsdb"
)

type DenormalizationRule struct {
	Metric   string
	TagNames []string
}

func (d *DenormalizationRule) String() string {
	inputTags, outputTags := "", ""
	val := 'a'
	for i, tagk := range d.TagNames {
		if i != 0 {
			inputTags += ","
			outputTags += "."
		}
		inputTags += fmt.Sprintf("%s=%s", tagk, string(val))
		outputTags += fmt.Sprintf("%s", string(val))
		val++
	}
	return fmt.Sprintf("%s{%s} -> __%s.%s", d.Metric, inputTags, outputTags, d.Metric)
}

var denormalizationRules map[string]*DenormalizationRule

func parseDenormalizationRules() {
	if *denormalize == "" {
		return
	}
	denormalizationRules = make(map[string]*DenormalizationRule)
	rules := strings.Split(*denormalize, ",")
	for _, r := range rules {
		parts := strings.Split(r, "__")
		if len(parts) < 2 {
			log.Fatal("Denormalization rules must have at least one tag name specified.")
		}
		rule := &DenormalizationRule{Metric: parts[0]}
		for i := 1; i < len(parts); i++ {
			rule.TagNames = append(rule.TagNames, parts[i])
		}
		log.Println("Denormalizing", rule)
		denormalizationRules[rule.Metric] = rule
	}
}

func (d *DenormalizationRule) Translate(dp *opentsdb.DataPoint) (*opentsdb.DataPoint, error) {
	newDp := &opentsdb.DataPoint{Timestamp: dp.Timestamp, Value: dp.Value}
	tagString := "__"
	newDp.Tags = dp.Tags
	for i, tagName := range d.TagNames {
		val, ok := dp.Tags[tagName]
		if !ok {
			return nil, fmt.Errorf("tag %s not present in data point for %s.", tagName, dp.Metric)
		}
		if i > 0 {
			tagString += "."
		}
		tagString += val
	}
	newDp.Metric = tagString + "." + dp.Metric
	return newDp, nil
}
