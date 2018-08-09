package cache // import "bosun.org/cmd/bosun/cache"

import (
	"sync"

	"github.com/golang/groupcache/lru"
	"github.com/golang/groupcache/singleflight"
)

type Cache struct {
	g singleflight.Group

	sync.Mutex
	lru  *lru.Cache
	Name string
}

// New creates a new LRU cache of the request length with
// an exported Name for instrumentation
func New(name string, MaxEntries int) *Cache {
	return &Cache{
		lru:  lru.New(MaxEntries),
		Name: name,
	}
}

// Get returns a cached value based on the passed key or runs the passed function to get the value
// if there is no corresponding value in the cache
func (c *Cache) Get(key string, getFn func() (interface{}, error)) (i interface{}, err error, hit bool) {
	if c == nil {
		i, err = getFn()
		return
	}
	c.Lock()
	result, ok := c.lru.Get(key)
	c.Unlock()
	if ok {
		return result, nil, true
	}
	// our lock only serves to protect the lru.
	// we can (and should!) do singleflight requests concurrently
	i, err = c.g.Do(key, func() (interface{}, error) {
		v, err := getFn()
		if err == nil {
			c.Lock()
			c.lru.Add(key, v)
			c.Unlock()
		}
		return v, err
	})
	return
}
