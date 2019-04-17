package sched

import (
	"encoding/json"

	"bosun.org/cmd/bosun/expr"
	elastic "github.com/olivere/elastic"
)

func (c *Context) esQuery7(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) interface{} {
	newFilter := expr.ScopeES7(c.Group(), filter.Query(expr.ESV7).(elastic.Query))
	req, err := expr.ESBaseQuery7(c.runHistory.Start, indexRoot, newFilter, sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := c.runHistory.Backends.ElasticHosts.Query7(req)
	if err != nil {
		c.addError(err)
		return nil
	}
	r := make([]interface{}, len(results.Hits.Hits))
	for i, h := range results.Hits.Hits {
		var err error
		err = json.Unmarshal(h.Source, &r[i])
		if err != nil {
			c.addError(err)
			return nil
		}
	}
	return r
}

func (c *Context) esQueryAll7(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) interface{} {
	req, err := expr.ESBaseQuery7(c.runHistory.Start, indexRoot, filter.Query(expr.ESV7).(elastic.Query), sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := c.runHistory.Backends.ElasticHosts.Query7(req)
	if err != nil {
		c.addError(err)
		return nil
	}
	r := make([]interface{}, len(results.Hits.Hits))
	for i, h := range results.Hits.Hits {
		var err error
		err = json.Unmarshal(h.Source, &r[i])
		if err != nil {
			c.addError(err)
			return nil
		}
	}
	return r
}
