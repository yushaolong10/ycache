package ycache

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

var errLruNotFoundKey = errors.New("lru key not exist")

type simpleLRU struct {
	mutex    sync.Mutex
	maxCount int64                    //max cache key counts
	ttl      int64                    //seconds ttl
	lruList  *list.List               //list
	lruMap   map[string]*list.Element //map

	reqCount int64 //request counts
	hitCount int64 //hit counts
	keyCount int64 //current cache key counts
}

type lruEntry struct {
	key         string
	value       Indicator
	createAt    int64 //create unix timestamp
	updateCount int64
}

func newSimpleLRU(maxCount int64, ttl int64) *simpleLRU {
	cache := &simpleLRU{
		maxCount: maxCount,
		ttl:      ttl,
		lruList:  list.New(),
		lruMap:   make(map[string]*list.Element),
	}
	return cache
}

//Update return key total update upCount
func (cache *simpleLRU) Update(key string, value Indicator) int64 {
	cache.mutex.Lock()
	defer func() {
		cache.checkWithLocked()
		cache.mutex.Unlock()
	}()
	if ele, ok := cache.lruMap[key]; ok { //exist
		item := ele.Value.(*lruEntry)
		item.value = value
		item.createAt = time.Now().Unix()
		item.updateCount++
		cache.lruList.MoveToBack(ele)
		return item.updateCount
	} else { //new
		item := &lruEntry{
			key:         key,
			value:       value,
			createAt:    time.Now().Unix(),
			updateCount: 1,
		}
		cache.lruMap[key] = cache.lruList.PushBack(item)
		cache.keyCount++
		return item.updateCount
	}
}

//Get for get key value
func (cache *simpleLRU) Get(key string) (Indicator, error) {
	cache.mutex.Lock()
	defer func() {
		cache.checkWithLocked()
		cache.mutex.Unlock()
	}()
	cache.reqCount++
	if ele, ok := cache.lruMap[key]; ok {
		item := ele.Value.(*lruEntry)
		if item.createAt+cache.ttl > time.Now().Unix() { //有效
			cache.hitCount++
			cache.lruList.MoveToBack(ele)
			return item.value, nil
		}
		//expire
		cache.lruList.Remove(ele)
		delete(cache.lruMap, item.key)
		cache.keyCount--
	}
	return nil, errLruNotFoundKey
}

//Delete from delete key
func (cache *simpleLRU) Delete(key string) error {
	cache.mutex.Lock()
	defer func() {
		cache.checkWithLocked()
		cache.mutex.Unlock()
	}()
	if ele, ok := cache.lruMap[key]; ok {
		item := ele.Value.(*lruEntry)
		cache.lruList.Remove(ele)
		delete(cache.lruMap, item.key)
		cache.keyCount--
		return nil
	}
	return errLruNotFoundKey
}

func (cache *simpleLRU) checkWithLocked() {
	now := time.Now().Unix()
	for cache.lruList.Front() != nil {
		front := cache.lruList.Front()
		e := front.Value.(*lruEntry)
		//key count not greater and key not expired
		if cache.keyCount <= cache.maxCount && now < e.createAt+cache.ttl {
			break
		}
		cache.lruList.Remove(front)
		delete(cache.lruMap, e.key)
		cache.keyCount--
	}
}

func (cache *simpleLRU) GetReqCount() int64 {
	return cache.reqCount
}

func (cache *simpleLRU) GetHitCount() int64 {
	return cache.hitCount
}

func (cache *simpleLRU) GetKeysCount() int64 {
	return cache.keyCount
}
