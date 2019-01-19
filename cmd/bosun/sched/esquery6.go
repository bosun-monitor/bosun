package sched

import (
	"encoding/json"

	"bosun.org/cmd/bosun/expr/tsdbs"
	esExpr "bosun.org/cmd/bosun/expr/tsdbs/elastic"

	elastic "github.com/olivere/elastic"
)

func (c *Context) esQuery6(indexRoot tsdbs.ESIndexer, filter tsdbs.ESQuery, sduration, eduration string, size int) interface{} {
	newFilter := esExpr.ScopeES6(c.Group(), filter.Query(string(esExpr.ESV6)).(elastic.Query))
	req, err := esExpr.ESBaseQuery6(c.runHistory.Start, indexRoot, newFilter, sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := esExpr.Query6(req, c.runHistory.TSDBs.Elastic)
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

func (c *Context) esQueryAll6(indexRoot tsdbs.ESIndexer, filter tsdbs.ESQuery, sduration, eduration string, size int) interface{} {
	req, err := esExpr.ESBaseQuery6(c.runHistory.Start, indexRoot, filter.Query(string(esExpr.ESV6)).(elastic.Query), sduration, eduration, size, c.ElasticHost)
	if err != nil {
		c.addError(err)
		return nil
	}
	results, err := esExpr.Query6(req, c.runHistory.TSDBs.Elastic)
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
