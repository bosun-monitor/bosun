package denormalize

import (
	"fmt"
	"log"
	"strings"

	"bosun.org/opentsdb"
)

// DenormalizationRule can convert a metric name based on selected tags
type DenormalizationRule struct {
	metric   string
	tagNames []string
}

const (
	tagsSeparator  = "__"
	rulesSeparator = ","
)

func (d *DenormalizationRule) String() string {
	inputTags, outputTags := "", ""
	val := 'a'
	for i, tagk := range d.tagNames {
		if i != 0 {
			inputTags += ","
			outputTags += "."
		}
		inputTags += fmt.Sprintf("%s=%s", tagk, string(val))
		outputTags += string(val)
		val++
	}
	return fmt.Sprintf("%s{%s} -> __%s.%s", d.metric, inputTags, outputTags, d.metric)
}

// ParseDenormalizationRules parses a DenormalizationRule from a given string
//
// The expected format is a comma-seperated list of `metric__tagname__tagname` rules.
func ParseDenormalizationRules(config string) (map[string]*DenormalizationRule, error) {
	m := make(map[string]*DenormalizationRule)
	rules := strings.Split(config, rulesSeparator)
	for _, r := range rules {
		parts := strings.Split(r, tagsSeparator)
		if len(parts) < 2 {
			return nil, fmt.Errorf("denormalization rules must have at least one tag name specified")
		}
		rule := &DenormalizationRule{metric: parts[0]}
		rule.tagNames = append(rule.tagNames, parts[1:]...)
		log.Println("Denormalizing", rule)
		m[rule.metric] = rule
	}
	return m, nil
}

// Convert modifies a given `opentsdb.DataPoint` in-place based on the `DenormalizationRule` `d`
//
// For n tag names in `d`, it sets the metric name to  `__tagvalue1.tagvalue2.(...).tagvalueN.metric`".
func (d *DenormalizationRule) Convert(dp *opentsdb.DataPoint) error {
	selectedTagValues := make([]string, 0)
	for _, tagName := range d.tagNames {
		val, ok := dp.Tags[tagName]
		if !ok {
			return fmt.Errorf("tag %s not present in data point for %s", tagName, dp.Metric)
		}
		selectedTagValues = append(selectedTagValues, val)
	}
	dp.Metric = fmt.Sprintf("%s%s.%s", tagsSeparator, strings.Join(selectedTagValues, "."), dp.Metric)
	return nil
}
