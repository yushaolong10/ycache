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

type LoadFunc func(ctx context.Context, key string) ([]byte, error)

type BatchLoadFunc func(ctx context.Context, keys []string) (map[string][]byte, error)

type YInstance struct {
	root      string
	name      string
	ttl       int
	factor    int
	random    int
	lc        *loadControl
	cacheList []ICache
	collector ICollector
	errHandle ErrorHandleFunc
	stat      *YStat
}

func (yi *YInstance) Get(ctx context.Context, prefix string, key string, loadFn LoadFunc) (value []byte, err error) {
	defer func() {
		yi.updateIndicator(ctx, prefix, key, value, loadFn)
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	realKey := yi.addPrefix(prefix, key)
	for index := 0; index < len(yi.cacheList); index++ {
		value, err = yi.getFromCache(index, ctx, realKey)
		if err == nil {
			for head := 0; head < index; head++ {
				_ = yi.setToCache(head, ctx, realKey, value)
			}
			return value, nil
		}
	}
	var done func()
	done, value, err = yi.loadFromSource(ctx, prefix, key, loadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	}
	if err != nil {
		return nil, err
	}
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.setToCache(index, ctx, realKey, value)
	}
	return value, nil
}

func (yi *YInstance) BatchGet(ctx context.Context, prefix string, keys []string, batchLoadFn BatchLoadFunc) (kvs map[string][]byte, err error) {
	defer func() {
		yi.batchUpdateIndicator(ctx, prefix, keys, kvs, batchLoadFn)
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	realKeyList := make([]string, 0)
	keyMap := make(map[string]string)
	for _, key := range keys {
		realKey := yi.addPrefix(prefix, key)
		realKeyList = append(realKeyList, realKey)
		//record real key to request key
		keyMap[realKey] = key
	}
	//data key is request key.
	dataKvs := make(map[string][]byte)
	for index := 0; index < len(yi.cacheList); index++ {
		realKvs, err := yi.batchGetFromCache(index, ctx, realKeyList)
		if err == nil {
			//this level values not exist
			if len(realKvs) == 0 {
				continue
			}
			newRealKvs := make(map[string][]byte)
			for k, v := range realKvs {
				newRealKvs[k] = v
				//note: transfer to request key
				dataKvs[keyMap[k]] = v
			}
			for head := 0; head < index; head++ {
				_ = yi.batchSetToCache(head, ctx, newRealKvs)
			}
		}
	}
	if len(dataKvs) == len(keys) {
		return dataKvs, nil
	}
	loadKeys := make([]string, 0)
	for _, key := range keys {
		if _, ok := dataKvs[key]; !ok {
			loadKeys = append(loadKeys, key)
		}
	}
	var done func()
	done, loadKvs, err := yi.batchLoadFromSource(ctx, prefix, loadKeys, batchLoadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	}
	if err != nil {
		return nil, err
	}
	if len(loadKvs) == 0 {
		return dataKvs, nil
	}
	loadRealKvs := make(map[string][]byte)
	for key, value := range loadKvs {
		//add to data
		dataKvs[key] = value
		//to save new key
		realKey := yi.addPrefix(prefix, key)
		loadRealKvs[realKey] = value
	}
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.batchSetToCache(index, ctx, loadRealKvs)
	}
	return dataKvs, nil
}

func (yi *YInstance) Delete(ctx context.Context, prefix string, key string) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	realKey := yi.addPrefix(prefix, key)
	for index := len(yi.cacheList) - 1; index >= 0; index-- {
		_ = yi.delToCache(index, ctx, realKey)
	}
	return nil
}

func (yi *YInstance) BatchDelete(ctx context.Context, prefix string, keys []string) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
		}
	}()
	realKeyList := make([]string, 0)
	for _, key := range keys {
		realKey := yi.addPrefix(prefix, key)
		realKeyList = append(realKeyList, realKey)
	}
	for index := len(yi.cacheList) - 1; index >= 0; index-- {
		_ = yi.batchDelToCache(index, ctx, realKeyList)
	}
	return nil
}

func (yi *YInstance) Update(ctx context.Context, prefix string, key string, loadFn LoadFunc) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		atomic.AddInt64(&yi.stat.TotalUpdateCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
			atomic.AddInt64(&yi.stat.TotalUpdateFailed, 1)
		}
	}()
	var done func()
	done, value, err := yi.loadFromSource(ctx, prefix, key, loadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	}
	if err != nil {
		return err
	}
	realKey := yi.addPrefix(prefix, key)
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.setToCache(index, ctx, realKey, value)
	}
	return err
}

func (yi *YInstance) BatchUpdate(ctx context.Context, prefix string, keys []string, batchLoadFn BatchLoadFunc) (err error) {
	defer func() {
		atomic.AddInt64(&yi.stat.TotalReqCount, 1)
		atomic.AddInt64(&yi.stat.TotalUpdateCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.TotalReqFailed, 1)
			atomic.AddInt64(&yi.stat.TotalUpdateFailed, 1)
		}
	}()
	var done func()
	done, loadKvs, err := yi.batchLoadFromSource(ctx, prefix, keys, batchLoadFn)
	if done != nil {
		//when write to cache successfully, call done
		defer done()
	}
	if err != nil {
		return err
	}
	realKvs := make(map[string][]byte)
	for key, value := range loadKvs {
		realKey := yi.addPrefix(prefix, key)
		realKvs[realKey] = value
	}
	for index := 0; index < len(yi.cacheList); index++ {
		_ = yi.batchSetToCache(index, ctx, realKvs)
	}
	return nil
}

type YStat struct {
	TotalReqCount       int64
	TotalReqFailed      int64
	TotalUpdateCount    int64
	TotalUpdateFailed   int64
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
		TotalReqCount:       yi.stat.TotalReqCount,
		TotalReqFailed:      yi.stat.TotalReqFailed,
		TotalUpdateCount:    yi.stat.TotalUpdateCount,
		TotalUpdateFailed:   yi.stat.TotalUpdateFailed,
		TotalLoadConcurrent: yi.stat.TotalLoadConcurrent,
		TotalLoadCount:      yi.stat.TotalLoadCount,
		TotalLoadFailed:     yi.stat.TotalLoadFailed,
		CacheStats:          make(map[string]*CacheStat),
	}
	for name, cs := range yi.stat.CacheStats {
		stat.CacheStats[name] = &CacheStat{
			ReqCount:  cs.ReqCount,
			ReqFailed: cs.ReqFailed,
		}
	}
	return stat
}

func (yi *YInstance) getFromCache(index int, ctx context.Context, key string) (value []byte, err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	return cache.Get(ctx, key)
}

func (yi *YInstance) batchGetFromCache(index int, ctx context.Context, keys []string) (kvs map[string][]byte, err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	return cache.BatchGet(ctx, keys)
}

func (yi *YInstance) delToCache(index int, ctx context.Context, key string) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	return cache.Del(ctx, key)
}

func (yi *YInstance) batchDelToCache(index int, ctx context.Context, keys []string) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	return cache.BatchDel(ctx, keys)
}

func (yi *YInstance) setToCache(index int, ctx context.Context, key string, value []byte) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	ttl := yi.makeLevelTtl(index)
	return cache.Set(ctx, key, value, ttl)
}

func (yi *YInstance) batchSetToCache(index int, ctx context.Context, kvs map[string][]byte) (err error) {
	cache := yi.cacheList[index]
	defer func() {
		atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqCount, 1)
		if err != nil {
			atomic.AddInt64(&yi.stat.CacheStats[cache.Name()].ReqFailed, 1)
			yi.handleError(cache.Name(), err)
		}
	}()
	ttl := yi.makeLevelTtl(index)
	return cache.BatchSet(ctx, kvs, ttl)
}

func (yi *YInstance) loadFromSource(ctx context.Context, prefix string, key string, loadFn LoadFunc) (done func(), value []byte, err error) {
	if loadFn == nil {
		return nil, nil, ErrLoadFuncEmpty
	}
	defer func() {
		atomic.AddInt64(&yi.stat.TotalLoadConcurrent, 1)
	}()
	id := yi.addPrefix(prefix, key)
	lh := yi.lc.get(id)
	retVal, retErr, success := lh.raceRequest(func() (interface{}, error) {
		retVal, retErr := loadFn(ctx, key)
		return retVal, retErr
	})
	if success {
		done = func() {
			//add metric
			atomic.AddInt64(&yi.stat.TotalLoadCount, 1)
			if err != nil {
				atomic.AddInt64(&yi.stat.TotalLoadFailed, 1)
				yi.handleError("load", err)
			}
			yi.lc.delete(id)
			lh.wakeupWaiters(retVal, retErr)
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

func (yi *YInstance) batchLoadFromSource(ctx context.Context, prefix string, keys []string, batchLoadFn BatchLoadFunc) (done func(), kvs map[string][]byte, err error) {
	if batchLoadFn == nil {
		return nil, nil, ErrBatchLoadFuncEmpty
	}
	defer func() {
		atomic.AddInt64(&yi.stat.TotalLoadConcurrent, 1)
	}()
	sort.Sort(sort.StringSlice(keys))
	id := yi.addPrefix(prefix, strings.Join(keys, ","))
	lh := yi.lc.get(id)
	retVal, retErr, success := lh.raceRequest(func() (interface{}, error) {
		retVal, retErr := batchLoadFn(ctx, keys)
		return retVal, retErr
	})
	if success {
		done = func() {
			//add metric
			atomic.AddInt64(&yi.stat.TotalLoadCount, 1)
			if err != nil {
				atomic.AddInt64(&yi.stat.TotalLoadFailed, 1)
				yi.handleError("batchLoad", err)
			}
			yi.lc.delete(id)
			lh.wakeupWaiters(retVal, retErr)
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

func (yi *YInstance) addPrefix(prefix string, key string) string {
	if prefix != "" {
		return fmt.Sprintf("%s_%s_%s_%s", yi.root, yi.name, prefix, key)
	} else {
		return fmt.Sprintf("%s_%s_%s", yi.root, yi.name, key)
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

func (yi *YInstance) updateIndicator(ctx context.Context, prefix string, key string, value []byte, loadFn LoadFunc) {
	if yi.collector != nil && value != nil && loadFn != nil {
		realKey := yi.addPrefix(prefix, key)
		indicator := newYIndicator(realKey, prefix, key, loadFn, nil)
		_ = yi.collector.UpdateIndicators(ctx, yi.name, []Indicator{indicator})
	}
}

func (yi *YInstance) batchUpdateIndicator(ctx context.Context, prefix string, keys []string, kvs map[string][]byte, batchLoadFn BatchLoadFunc) {
	if yi.collector != nil && len(kvs) > 0 && batchLoadFn != nil {
		indicators := make([]Indicator, 0)
		for _, key := range keys {
			if _, ok := kvs[key]; ok {
				realKey := yi.addPrefix(prefix, key)
				indicator := newYIndicator(realKey, prefix, key, nil, batchLoadFn)
				indicators = append(indicators, indicator)
			}
		}
		_ = yi.collector.UpdateIndicators(ctx, yi.name, indicators)
	}
}

func (yi *YInstance) handleError(desc string, err error) {
	if yi.errHandle != nil {
		yi.errHandle(fmt.Errorf("yinstance(%s) %s err:%s", yi.name, desc, err.Error()))
	}
}

//loadControl for control all key request load func in concurrency condition
type loadControl struct {
	mutex      sync.Mutex
	keyHandler map[string]*loadHandler
}

func (lc *loadControl) get(key string) *loadHandler {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	if item, ok := lc.keyHandler[key]; ok {
		return item
	}
	item := &loadHandler{
		result: make(chan *loadResult, 0),
	}
	lc.keyHandler[key] = item
	return item
}

func (lc *loadControl) delete(key string) {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	if _, ok := lc.keyHandler[key]; ok {
		delete(lc.keyHandler, key)
	}
}

//loadHandler for handle the key in concurrent condition
type loadHandler struct {
	count  int64
	result chan *loadResult
}

type loadResult struct {
	value interface{}
	err   error
}

func (lh *loadHandler) raceRequest(fn func() (interface{}, error)) (value interface{}, err error, success bool) {
	number := atomic.AddInt64(&lh.count, 1)
	//only first request can call load func successfully
	//others will wait for reply
	if number == 1 {
		value, err = fn()
		success = true
	} else {
		ret := <-lh.result
		if ret == nil {
			err = ErrConcurrentLoad
		} else {
			value, err = ret.value, ret.err
		}
	}
	return
}

func (lh *loadHandler) wakeupWaiters(value interface{}, err error) {
	total := atomic.LoadInt64(&lh.count)
	for total > 1 {
		total--
		lh.result <- &loadResult{
			value: value,
			err:   err,
		}
	}
	close(lh.result)
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

func WithInstanceOptionCollector(collector ICollector) InstanceOption {
	return func(yc *YCache, yi *YInstance) error {
		_ = collector.RegisterHandler(yi.name, yi, yi.getBoundaryTtl())
		yi.collector = collector
		return nil
	}
}
