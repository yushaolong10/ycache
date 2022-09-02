package ycache

import (
	"fmt"
)

const (
	//default level ttl factor 10 times larger
	cacheTtlFactor = 10
	//default instance ttl 60 seconds
	cacheTtl = 60
)

func NewYCache(name string, opts ...CacheOption) *YCache {
	yc := &YCache{
		name:   name,
		caches: make(map[CacheLevel]ICache, 3),
	}
	for _, opt := range opts {
		opt(yc)
	}
	return yc
}

type ErrorHandleFunc func(err error)

type YCache struct {
	name      string
	errHandle ErrorHandleFunc
	caches    map[CacheLevel]ICache
}

func (yc *YCache) CreateInstance(name string, levels []CacheLevel, opts ...InstanceOption) (*YInstance, error) {
	yi := &YInstance{
		root:      yc.name,
		name:      name,
		ttl:       cacheTtl,
		factor:    cacheTtlFactor,
		cacheList: make([]ICache, 0),
		errHandle: yc.errHandle,
		lc: &loadControl{
			keyMap: make(map[string]*loadKey),
		},
		stat: &YStat{
			CacheStats: make(map[string]*CacheStat),
		},
	}
	for _, level := range levels {
		if cache, ok := yc.caches[level]; ok {
			yi.cacheList = append(yi.cacheList, cache)
			yi.stat.CacheStats[cache.Name()] = new(CacheStat)
		} else {
			return nil, fmt.Errorf("level(%d) cache not exist", level)
		}
	}
	for _, opt := range opts {
		if err := opt(yc, yi); err != nil {
			return nil, err
		}
	}
	return yi, nil
}

type CacheLevel int

const (
	CacheL1 CacheLevel = 1
	CacheL2 CacheLevel = 2
	CacheL3 CacheLevel = 3
)

type CacheOption func(yc *YCache)

func WithCacheOptionCacheLevel(level CacheLevel, cache ICache) CacheOption {
	return func(yc *YCache) {
		yc.caches[level] = cache
	}
}

func WithCacheOptionErrorHandle(handle ErrorHandleFunc) CacheOption {
	return func(yc *YCache) {
		yc.errHandle = handle
	}
}
