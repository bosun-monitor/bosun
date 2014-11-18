package expr

import (
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/opentsdb"
	"github.com/bosun-monitor/bosun/search"
)

type Lookup struct {
	Tags    []string
	Entries []*Entry
}

type Entry struct {
	AlertKey AlertKey
	Values   map[string]string
}

func (lookup *Lookup) Get(key string, tag opentsdb.TagSet) (value string, ok bool) {
	for _, entry := range lookup.Entries {
		value, ok = entry.Values[key]
		if !ok {
			continue
		}
		match := true
		for ak, av := range entry.AlertKey.Group() {
			matches, err := search.Match(av, []string{tag[ak]})
			if err != nil {
				return "", false
			}
			if len(matches) == 0 {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		return
	}
	return "", false
}
