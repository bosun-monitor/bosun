package expr

import (
	"strings"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
)

type AlertKey string

func NewAlertKey(name string, group opentsdb.TagSet) AlertKey {
	return AlertKey(name + group.String())
}

func (a AlertKey) Name() string {
	return strings.SplitN(string(a), "{", 2)[0]
}

func (a AlertKey) Group() opentsdb.TagSet {
	s := strings.SplitN(string(a), "{", 2)[1]
	s = s[:len(s)-1]
	if s == "" {
		return nil
	}
	g, err := opentsdb.ParseTags(s)
	if err != nil {
		panic(err)
	}
	return g
}

type AlertKeys []AlertKey

func (a AlertKeys) Len() int           { return len(a) }
func (a AlertKeys) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a AlertKeys) Less(i, j int) bool { return a[i] < a[j] }
