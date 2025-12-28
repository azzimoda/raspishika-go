package cache

import (
	"github.com/azzimoda/raspishika-go/internal/config"

	"github.com/patrickmn/go-cache"
)

type CacheProvider interface {
	Cache() *Cache
}

type Cache struct {
	C *cache.Cache
}

func New() *Cache {
	ttl := config.DefaultTTLDur()
	cache := cache.New(ttl, ttl*2)
	return &Cache{C: cache}
}
