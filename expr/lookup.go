package expr

type Lookup struct {
	Tags    []string
	Entries []*Entry
}

type Entry struct {
	AlertKey AlertKey
	Values   map[string]string
}
