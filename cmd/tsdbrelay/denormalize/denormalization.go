package denormalize

import (
	"fmt"
	"log"
	"strings"

	"bosun.org/opentsdb"
)

// DenormalizationRule can convert a metric name based on selected tags
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

// ParseDenormalizationRules parses a DenormalizationRule from a given string
//
// The expected format is a comma-seperated list of `metric__tagname__tagname` rules.
func ParseDenormalizationRules(config string) (map[string]*DenormalizationRule, error) {
	m := make(map[string]*DenormalizationRule)
	rules := strings.Split(config, ",")
	for _, r := range rules {
		parts := strings.Split(r, "__")
		if len(parts) < 2 {
			return nil, fmt.Errorf("denormalization rules must have at least one tag name specified")
		}
		rule := &DenormalizationRule{Metric: parts[0]}
		for _, part := range parts[1:] {
			rule.TagNames = append(rule.TagNames, part)
		}
		log.Println("Denormalizing", rule)
		m[rule.Metric] = rule
	}
	return m, nil
}

// Translate modifies a given `opentsdb.DataPoint` in-place based on the `DenormalizationRule` `d`
//
// For n tag names in `d`, it sets the metric name to  `__tagvalue1.tagvalue2.(...).tagvalueN.metric`".
func (d *DenormalizationRule) Translate(dp *opentsdb.DataPoint) error {
	tagString := "__"
	for i, tagName := range d.TagNames {
		val, ok := dp.Tags[tagName]
		if !ok {
			return fmt.Errorf("tag %s not present in data point for %s", tagName, dp.Metric)
		}
		if i > 0 {
			tagString += "."
		}
		tagString += val
	}
	dp.Metric = tagString + "." + dp.Metric

	return nil
}
