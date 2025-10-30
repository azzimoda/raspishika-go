package cache

import (
	"github.com/azzimoda/raspishika-go/internal/config"

	"github.com/patrickmn/go-cache"
)

type Cache struct {
	Config *config.CacheConfig
	C      *cache.Cache
}

func New(cfg *config.CacheConfig) *Cache {
	ttl := cfg.DefaultTTLDuration()
	cache := cache.New(ttl, ttl*2)
	return &Cache{
		Config: cfg,
		C:      cache,
	}
}
