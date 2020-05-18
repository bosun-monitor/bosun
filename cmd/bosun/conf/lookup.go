package conf

import (
	"bosun.org/cmd/bosun/search"
	"bosun.org/models"
	"bosun.org/opentsdb"
)

// ExprLookup implements the actual lookup functionality
// TODO: remove this and merge it with Lookup
type ExprLookup struct {
	Tags    []string
	Entries []*ExprEntry
}

// ExprEntry is an entry in a lookup, consisting of a set of tag keys and patterns to match against and a values map
//
// `AlertKey` is misleading. For lookups, it'll always only consist of tags, so isn't a valid alert key.
type ExprEntry struct {
	// TODO: Rename AlertKey
	AlertKey models.AlertKey
	Values   map[string]string
}

// Get searches the entries in a lookup for the given key and tag set
//
//    lookup host_base_contact {
//        entry host=nyhq-|-int|den-*|lon-* {
//            main_contact = it
//            chat_contact = it-chat
//        }
//        entry host=* {
//            main_contact = default
//        }
//    }
//
// Finds the first entry that has the targetKey in question (e.g. `chat_contact` in the above example) for which all
// tags specified in the entry also match the passed in tags of the alert.
//
// For the lookup in the above example:
// `Get("main_contact", {"host=foo"})` returns `(default, true)`.
// `Get("main_contact", {"host=lon-bar"})` returns `("it", true)`
// `Get("chat_contact", {"host=foo"})` returns `("", false)` (`host=*` matches, but doesn't have the target key)
// `Get("chat_contact", {"foo=bar"})` returns `("", false)` (no entry matches)
func (lookup *ExprLookup) Get(key string, tag opentsdb.TagSet) (value string, ok bool) {
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
