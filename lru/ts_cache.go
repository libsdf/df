/*
Package lru currently implements a type of time sensitive cache.
There is no real LRU implementations here, for now.
*/
package lru

import (
	"context"
	"sync"
	"time"
)

type item struct {
	lastUsedAt time.Time
	entry      interface{}
}

type Options struct {
	EntryMaxTTL time.Duration
}

type TSCache struct {
	items   *sync.Map
	options *Options
}

func (t *TSCache) Put(key string, entry interface{}) {
	t.items.Store(key, &item{
		lastUsedAt: time.Now(),
		entry:      entry,
	})
}

func (t *TSCache) Get(key string) (interface{}, bool) {
	if v, found := t.items.Load(key); found {
		if item, ok := v.(*item); ok {
			item.lastUsedAt = time.Now()
			return item.entry, true
		}
	}
	return nil, false
}

func (t *TSCache) removeOutdatedItems() {
	maxTTL := t.options.EntryMaxTTL
	keys := []string{}
	now := time.Now()
	t.items.Range(func(key, value any) bool {
		k := key.(string)
		if item, ok := value.(*item); ok {
			if item.lastUsedAt.Add(maxTTL).Before(now) {
				keys = append(keys, k)
			}
		} else {
			keys = append(keys, k)
		}
		return true
	})
	for _, k := range keys {
		t.items.Delete(k)
	}
}

func (t *TSCache) Worker(x context.Context) {
	for {
		select {
		case <-x.Done():
			return
		case <-time.After(time.Second * 15):
			t.removeOutdatedItems()
		}
	}
}

func NewTSCache(ttl time.Duration) *TSCache {
	return &TSCache{
		items:   new(sync.Map),
		options: &Options{EntryMaxTTL: ttl},
	}
}
