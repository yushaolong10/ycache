package ycache

import (
	"context"
	"fmt"
	"github.com/coocood/freecache"
	"runtime/debug"
	"sync"
)

func NewMemCache(name string, size int, gcPercent int) *MemCache {
	cache := freecache.NewCache(size)
	if gcPercent > 0 {
		debug.SetGCPercent(gcPercent)
	}
	return &MemCache{name: name, cache: cache}
}

type MemCache struct {
	name  string
	cache *freecache.Cache
}

func (mc *MemCache) Name() string {
	return mc.name
}

func (mc *MemCache) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := mc.cache.Get([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("freecache get key(%s) err:%s", key, err.Error())
	}
	return value, nil
}

func (mc *MemCache) BatchGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	mp := &sync.Map{}
	wg := &sync.WaitGroup{}
	for _, key := range keys {
		wg.Add(1)
		k1 := key
		go func() {
			defer wg.Done()
			value, err := mc.cache.Get([]byte(k1))
			if err == nil {
				mp.Store(k1, value)
			}
		}()
	}
	wg.Wait()
	data := make(map[string][]byte)
	for _, key := range keys {
		if v, ok := mp.Load(key); ok {
			data[key] = v.([]byte)
		}
	}
	return data, nil
}

func (mc *MemCache) Del(ctx context.Context, key string) error {
	_ = mc.cache.Del([]byte(key))
	return nil
}

func (mc *MemCache) BatchDel(ctx context.Context, keys []string) error {
	for _, key := range keys {
		_ = mc.cache.Del([]byte(key))
	}
	return nil
}

func (mc *MemCache) Set(ctx context.Context, key string, value []byte, ttl int) error {
	err := mc.cache.Set([]byte(key), value, ttl)
	if err != nil {
		return fmt.Errorf("freecache set key(%s) err:%s", key, err.Error())
	}
	return nil
}

func (mc *MemCache) BatchSet(ctx context.Context, kvs map[string][]byte, ttl int) error {
	for key, value := range kvs {
		err := mc.cache.Set([]byte(key), value, ttl)
		if err != nil {
			return fmt.Errorf("freecache batch set key(%s) err:%s", key, err.Error())
		}
	}
	return nil
}
