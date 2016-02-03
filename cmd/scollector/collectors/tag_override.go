package collectors

import (
	"fmt"
	"regexp"

	"bosun.org/opentsdb"
)

type TagOverride struct {
	matchedTags map[string]*regexp.Regexp
	tags        opentsdb.TagSet
}

func (to *TagOverride) AddTagOverrides(sources map[string]string, t opentsdb.TagSet) error {
	if to.matchedTags == nil {
		to.matchedTags = make(map[string]*regexp.Regexp)
	}
	var err error
	for tag, re := range sources {
		to.matchedTags[tag], err = regexp.Compile(re)
		if err != nil {
			return fmt.Errorf("invalid regexp: %s error: %s", re, err)
		}
	}

	if to.tags == nil {
		to.tags = t.Copy()
	} else {
		to.tags = to.tags.Merge(t)
	}

	return nil
}

func (to *TagOverride) ApplyTagOverrides(t opentsdb.TagSet) {
	namedMatchGroup := make(map[string]string)
	for tag, re := range to.matchedTags {
		if v, ok := t[tag]; ok {
			matches := re.FindStringSubmatch(v)

			if len(matches) > 1 {
				for i, match := range matches[1:] {
					matchedTag := re.SubexpNames()[i+1]
					if match != "" && matchedTag != "" {
						namedMatchGroup[matchedTag] = match
					}
				}
			}
		}
	}

	for tag, v := range namedMatchGroup {
		t[tag] = v
	}

	for tag, v := range to.tags {
		if v == "" {
			delete(t, tag)
		} else {
			t[tag] = v
		}
	}
}
