package backend

import (
	"context"
	"github.com/libsdf/df/lru"
	"time"
)

var (
	dnsCache = lru.NewTSCache(time.Minute * 5)
)

func CacheWorker(x context.Context) {
	dnsCache.Worker(x)
}
