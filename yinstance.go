package ycache

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrLoadFuncEmpty      = fmt.Errorf("request LoadFunc is nil")
	ErrBatchLoadFuncEmpty = fmt.Errorf("request BatchLoadFunc is nil")
	ErrConcurrentLoad     = fmt.Errorf("concurrent load return nil")
)

type LoadFunc func(ctx context.Context, realKey string) ([]byte, error)

type BatchLoadFunc func(ctx context.Context, realKeys []string) (map[string][]byte, error)

type YInstance struct {
	root      string
	name      string
	ttl       int
	factor    int
	random    int
	lc        *loadControl
	cacheList []ICache
	errHandle ErrorHandleFunc
	stat      *YStat
}

func (yi *YInstance) Name() string {
	return yi.name
}

func (yi *YInstance) Get(ctx context.Context, prefix string, realKey string, loadFn LoadFunc) (value []byte, err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	cacheKey := yi.addPrefix(prefix, realKey)
	for index := 0; index < len(yi.cacheList); index++ {
		value, err = yi.getFromCache(ctx, index, cacheKey)
		if err == nil {
			for head := 0; head < index; head++ {
				_ = yi.setToCache(ctx, head, cacheKey, value)
			}
			return value, nil
		}
	}
	var done func()
	done, value, err = yi.loadFromSource(ctx, prefix, realKey, loadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	} else {
		//value from channel not need to set cache
		return value, err
	}
	if err != nil {
		return nil, err
	}
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.setToCache(ctx, index, cacheKey, value)
	}
	return value, nil
}

func (yi *YInstance) BatchGet(ctx context.Context, prefix string, realKeys []string, batchLoadFn BatchLoadFunc) (kvs map[string][]byte, err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	cacheKeyList := make([]string, 0)
	keyMap := make(map[string]string)
	for _, key := range realKeys {
		cacheKey := yi.addPrefix(prefix, key)
		cacheKeyList = append(cacheKeyList, cacheKey)
		//record real key to request key
		keyMap[cacheKey] = key
	}
	//data key is request key.
	dataKvs := make(map[string][]byte)
	for index := 0; index < len(yi.cacheList); index++ {
		cacheKvs, err := yi.batchGetFromCache(ctx, index, cacheKeyList)
		if err == nil {
			//this level values not exist
			if len(cacheKvs) == 0 {
				continue
			}
			newCacheKvs := make(map[string][]byte)
			for k, v := range cacheKvs {
				newCacheKvs[k] = v
				//note: transfer to request key
				dataKvs[keyMap[k]] = v
			}
			for head := 0; head < index; head++ {
				_ = yi.batchSetToCache(ctx, head, newCacheKvs)
			}
			if len(dataKvs) == len(realKeys) {
				return dataKvs, nil
			}
		}
	}
	loadRealKeys := make([]string, 0)
	for _, key := range realKeys {
		if _, ok := dataKvs[key]; !ok {
			loadRealKeys = append(loadRealKeys, key)
		}
	}
	var done func()
	done, loadRealKvs, err := yi.batchLoadFromSource(ctx, prefix, loadRealKeys, batchLoadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	} else {
		for key, value := range loadRealKvs {
			dataKvs[key] = value
		}
		//value from channel not need to set cache
		return dataKvs, err
	}
	if err != nil {
		return nil, err
	}
	if len(loadRealKvs) == 0 {
		return dataKvs, nil
	}
	loadCacheKvs := make(map[string][]byte)
	for key, value := range loadRealKvs {
		//add to data
		dataKvs[key] = value
		//to save new key
		cacheKey := yi.addPrefix(prefix, key)
		loadCacheKvs[cacheKey] = value
	}
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.batchSetToCache(ctx, index, loadCacheKvs)
	}
	return dataKvs, nil
}

func (yi *YInstance) Delete(ctx context.Context, prefix string, realKey string) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	cacheKey := yi.addPrefix(prefix, realKey)
	for index := len(yi.cacheList) - 1; index >= 0; index-- {
		_ = yi.delToCache(ctx, index, cacheKey)
	}
	return nil
}

func (yi *YInstance) BatchDelete(ctx context.Context, prefix string, realKeys []string) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	cacheKeyList := make([]string, 0)
	for _, key := range realKeys {
		cacheKey := yi.addPrefix(prefix, key)
		cacheKeyList = append(cacheKeyList, cacheKey)
	}
	for index := len(yi.cacheList) - 1; index >= 0; index-- {
		_ = yi.batchDelToCache(ctx, index, cacheKeyList)
	}
	return nil
}

type YStat struct {
	TotalReqCount       int64
	TotalReqFailed      int64
	TotalLoadConcurrent int64
	TotalLoadCount      int64
	TotalLoadFailed     int64
	CacheStats          map[string]*CacheStat
}

type CacheStat struct {
	ReqCount  int64
	ReqFailed int64
}

func (yi *YInstance) Stat() *YStat {
	stat := &YStat{
		TotalReqCount:       atomic.LoadInt64(&yi.stat.TotalReqCount),
		TotalReqFailed:      atomic.LoadInt64(&yi.stat.TotalReqFailed),
		TotalLoadConcurrent: atomic.LoadInt64(&yi.stat.TotalLoadConcurrent),
		TotalLoadCount:      atomic.LoadInt64(&yi.stat.TotalLoadCount),
		TotalLoadFailed:     atomic.LoadInt64(&yi.stat.TotalLoadFailed),
		CacheStats:          make(map[string]*CacheStat),
	}
	for name, cs := range yi.stat.CacheStats {
		stat.CacheStats[name] = &CacheStat{
			ReqCount:  atomic.LoadInt64(&cs.ReqCount),
			ReqFailed: atomic.LoadInt64(&cs.ReqFailed),
		}
	}
	return stat
}

func (yi *YInstance) getFromCache(ctx context.Context, index int, cacheKey string) (value []byte, err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("getFromCache(%s) failed", cache.Name()), err)
		}
	}()
	return cache.Get(ctx, cacheKey)
}

func (yi *YInstance) batchGetFromCache(ctx context.Context, index int, cacheKeys []string) (kvs map[string][]byte, err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("batchGetFromCache(%s) failed", cache.Name()), err)
		}
	}()
	return cache.BatchGet(ctx, cacheKeys)
}

func (yi *YInstance) delToCache(ctx context.Context, index int, cacheKey string) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("delToCache(%s) failed", cache.Name()), err)
		}
	}()
	return cache.Del(ctx, cacheKey)
}

func (yi *YInstance) batchDelToCache(ctx context.Context, index int, cacheKeys []string) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("batchDelToCache(%s) failed", cache.Name()), err)
		}
	}()
	return cache.BatchDel(ctx, cacheKeys)
}

func (yi *YInstance) setToCache(ctx context.Context, index int, cacheKey string, value []byte) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("setToCache(%s) failed", cache.Name()), err)
		}
	}()
	ttl := yi.makeLevelTtl(index)
	return cache.Set(ctx, cacheKey, value, ttl)
}

func (yi *YInstance) batchSetToCache(ctx context.Context, index int, cacheKvs map[string][]byte) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(fmt.Sprintf("batchSetToCache(%s) failed", cache.Name()), err)
		}
	}()
	ttl := yi.makeLevelTtl(index)
	return cache.BatchSet(ctx, cacheKvs, ttl)
}

func (yi *YInstance) loadFromSource(ctx context.Context, prefix string, realKey string, loadFn LoadFunc) (done func(), value []byte, err error) {
	if loadFn == nil {
		return nil, nil, ErrLoadFuncEmpty
	}
	defer func() {
		atomic.AddInt64(&yi.stat.TotalLoadConcurrent, 1)
	}()
	raceId := yi.addPrefix(prefix, realKey)
	rh := yi.lc.get(raceId)
	retVal, retErr, success := rh.request(func() (interface{}, error) {
		lv, le := loadFn(ctx, realKey)
		return lv, le
	})
	if success {
		done = func() {
			//add metric
			atomic.AddInt64(&yi.stat.TotalLoadCount, 1)
			if err != nil {
				atomic.AddInt64(&yi.stat.TotalLoadFailed, 1)
				yi.handleError("loadFromSource failed", err)
			}
			yi.lc.delete(raceId)
			rh.wakeupWaiters(retVal, retErr)
		}
	}
	if retVal != nil {
		value = retVal.([]byte)
	}
	if retErr != nil {
		err = retErr
	}
	return
}

func (yi *YInstance) batchLoadFromSource(ctx context.Context, prefix string, realKeys []string, batchLoadFn BatchLoadFunc) (done func(), kvs map[string][]byte, err error) {
	if batchLoadFn == nil {
		return nil, nil, ErrBatchLoadFuncEmpty
	}
	defer func() {
		atomic.AddInt64(&yi.stat.TotalLoadConcurrent, 1)
	}()
	sort.Strings(realKeys)
	raceId := yi.addPrefix(prefix, strings.Join(realKeys, ","))
	rh := yi.lc.get(raceId)
	retVal, retErr, success := rh.request(func() (interface{}, error) {
		lv, le := batchLoadFn(ctx, realKeys)
		return lv, le
	})
	if success {
		done = func() {
			//add metric
			atomic.AddInt64(&yi.stat.TotalLoadCount, 1)
			if err != nil {
				atomic.AddInt64(&yi.stat.TotalLoadFailed, 1)
				yi.handleError("batchLoadFromSource failed", err)
			}
			yi.lc.delete(raceId)
			rh.wakeupWaiters(retVal, retErr)
		}
	}
	if retVal != nil {
		kvs = retVal.(map[string][]byte)
	}
	if retErr != nil {
		err = retErr
	}
	return
}

func (yi *YInstance) addPrefix(prefix string, realKey string) string {
	if prefix != "" {
		return fmt.Sprintf("%s_%s_%s_%s", yi.root, yi.name, prefix, realKey)
	} else {
		return fmt.Sprintf("%s_%s_%s", yi.root, yi.name, realKey)
	}
}

func (yi *YInstance) makeLevelTtl(index int) int {
	ttl := yi.ttl * (1 + index*yi.factor)
	if yi.random > 0 {
		src := rand.NewSource(time.Now().UnixNano())
		number := rand.New(src).Intn(yi.random)
		ttl = ttl + number
	}
	return ttl
}

func (yi *YInstance) getBoundaryTtl() int {
	boundaryTtl := yi.ttl * (1 + (len(yi.cacheList)-1)*yi.factor)
	return boundaryTtl
}

func (yi *YInstance) handleError(desc string, err error) {
	if yi.errHandle != nil {
		yi.errHandle(fmt.Sprintf("[yinstance:%s] error:%s", yi.name, desc), err)
	}
}

//loadControl for control all key request load func in concurrency condition
type loadControl struct {
	mutex    sync.Mutex
	handlers map[string]*raceHandler
}

func (lc *loadControl) get(id string) *raceHandler {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	if item, ok := lc.handlers[id]; ok {
		return item
	}
	item := &raceHandler{
		result: make(chan *raceResult, 0),
	}
	lc.handlers[id] = item
	return item
}

func (lc *loadControl) delete(id string) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	if _, ok := lc.handlers[id]; ok {
		delete(lc.handlers, id)
	}
}

//raceHandler for handle the key in concurrent condition
type raceHandler struct {
	count  int64
	result chan *raceResult
}

type raceResult struct {
	value interface{}
	err   error
}

func (rh *raceHandler) request(fn func() (interface{}, error)) (value interface{}, err error, success bool) {
	number := atomic.AddInt64(&rh.count, 1)
	//only first request can call load func successfully
	//others will wait for reply
	if number == 1 {
		value, err = fn()
		success = true
	} else {
		ret := <-rh.result
		if ret == nil {
			err = ErrConcurrentLoad
		} else {
			value, err = ret.value, ret.err
		}
	}
	return
}

func (rh *raceHandler) wakeupWaiters(value interface{}, err error) {
	total := atomic.LoadInt64(&rh.count)
	for total > 1 {
		total--
		rh.result <- &raceResult{
			value: value,
			err:   err,
		}
	}
	close(rh.result)
}

type InstanceOption func(yc *YCache, yi *YInstance) error

func WithInstanceOptionRandomTtl(random int) InstanceOption {
	return func(yc *YCache, yi *YInstance) error {
		yi.random = random
		return nil
	}
}

func WithInstanceOptionCacheTtl(ttl int) InstanceOption {
	return func(yc *YCache, yi *YInstance) error {
		yi.ttl = ttl
		return nil
	}
}

func WithInstanceOptionTtlFactor(factor int) InstanceOption {
	return func(yc *YCache, yi *YInstance) error {
		yi.factor = factor
		return nil
	}
}
