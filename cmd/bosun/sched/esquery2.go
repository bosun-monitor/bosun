package sched

import (
	"encoding/json"

	"bosun.org/cmd/bosun/expr"
	elastic "gopkg.in/olivere/elastic.v3"
)

func (c *Context) esQuery2(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) interface{} {
	newFilter := expr.ScopeES2(c.Group(), filter.Query(expr.ESV2).(elastic.Query))
	req, err := expr.ESBaseQuery2(c.runHistory.Start, indexRoot, newFilter, sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := c.runHistory.Backends.ElasticHosts.Query2(req)
	if err != nil {
		c.addError(err)
		return nil
	}
	r := make([]interface{}, len(results.Hits.Hits))
	for i, h := range results.Hits.Hits {
		var err error
		err = json.Unmarshal(*h.Source, &r[i])
		if err != nil {
			c.addError(err)
			return nil
		}
	}
	return r
}

func (c *Context) esQueryAll2(indexRoot expr.ESIndexer, filter expr.ESQuery, sduration, eduration string, size int) interface{} {
	req, err := expr.ESBaseQuery2(c.runHistory.Start, indexRoot, filter.Query(expr.ESV2).(elastic.Query), sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := c.runHistory.Backends.ElasticHosts.Query2(req)
	if err != nil {
		c.addError(err)
		return nil
	}
	r := make([]interface{}, len(results.Hits.Hits))
	for i, h := range results.Hits.Hits {
		var err error
		err = json.Unmarshal(*h.Source, &r[i])
		if err != nil {
			c.addError(err)
			return nil
		}
	}
	return r
}
