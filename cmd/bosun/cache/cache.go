package cache // import "bosun.org/cmd/bosun/cache"

import (
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"
)

type Cache struct {
	g singleflight.Group

	sync.Mutex
	lru *lru.Cache
}

func New(MaxEntries int) *Cache {
	return &Cache{
		lru: lru.New(MaxEntries),
	}
}

func (c *Cache) Get(key string, getFn func() (interface{}, error)) (interface{}, error) {
	if c == nil {
		return getFn()
	}
	c.Lock()
	result, ok := c.lru.Get(key)
	c.Unlock()
	if ok {
		return result, nil
	}
	// our lock only serves to protect the lru.
	// we can (and should!) do singleflight requests concurently
	return c.g.Do(key, func() (interface{}, error) {
		v, err := getFn()
		if err == nil {
			c.Lock()
			c.lru.Add(key, v)
			c.Unlock()
		}
		return v, err
	})
}
