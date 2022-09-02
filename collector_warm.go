package ycache

import (
	"context"
	"fmt"
	"github.com/rfyiamcool/go-timewheel"
	"sync"
	"time"
)

type WarmCollectorConfig struct {
	BuffSeconds int64
	EntryNumber int64
	TimeRatio   int64
	MaxHotCount int64
	HotKeyTtl   int64
	NewContext  func() context.Context
}

func NewWarmCollector(conf *WarmCollectorConfig) *WarmCollector {
	//new time wheel
	tw, _ := timewheel.NewTimeWheel(1*time.Second, 3600)
	tw.Start()
	//create
	collector := &WarmCollector{
		bufferSecs: conf.BuffSeconds,
		entryTimes: conf.EntryNumber,
		timeRatio:  conf.TimeRatio,
		handlers:   make(map[string]*updateHandler),
		lru:        newSimpleLRU(conf.MaxHotCount, conf.HotKeyTtl),
		timeWheel:  tw,
		newCtx: func() context.Context {
			return context.Background()
		},
	}
	if conf.NewContext != nil {
		collector.newCtx = conf.NewContext
	}
	//invoke task
	tw.AddCron(time.Second*10, collector.invoke)
	return collector
}

//WarmCollector like cache read-allocate ways to pre warm cache before reach it's ttl.
type WarmCollector struct {
	bufferSecs int64 //buffer second time to become target indicator, recommend greater 30s.
	entryTimes int64 //the minimal times to entry time wheel
	timeRatio  int64 //at ratio time to trigger task, between 1-100
	lru        *simpleLRU
	handlers   map[string]*updateHandler
	timeWheel  *timewheel.TimeWheel
	newCtx     func() context.Context
}

type updateHandler struct {
	handler     IUpdateHandler
	boundaryTtl int64
	yf          *yFunc
	mList       *simpleMapList
}

func (wc *WarmCollector) RegisterHandler(instance string, handler IUpdateHandler, boundaryTtl int) error {
	wc.handlers[instance] = &updateHandler{
		handler:     handler,
		boundaryTtl: int64(boundaryTtl),
		yf: &yFunc{
			loadMap: make(map[string]*yLoad),
		},
		mList: newSimpleMapList(),
	}
	return nil
}

func (wc *WarmCollector) UpdateIndicators(ctx context.Context, instance string, indicators []Indicator) error {
	for _, indicator := range indicators {
		_ = wc.updateIndicator(ctx, instance, indicator)
	}
	return nil
}

func (wc *WarmCollector) updateIndicator(ctx context.Context, instance string, indicator Indicator) error {
	uHandler := wc.handlers[instance]
	//judgement buffer
	if uHandler.boundaryTtl < wc.bufferSecs {
		return nil
	}
	//add func
	uHandler.yf.addFunc(indicator.Prefix(), indicator.LoadFunc(), indicator.BatchLoadFunc())
	//for lru filter
	number := wc.lru.Update(indicator.Id())
	if number != wc.entryTimes {
		return nil
	}
	//in list not trigger again
	if uHandler.mList.Exist(indicator) {
		return nil
	}
	//here simply think current timestamp is the first create cache time
	//get preset ratio ttl
	presetTtl := int64(uHandler.boundaryTtl * wc.timeRatio / 100)
	//add to map list
	now := time.Now().Unix()
	//calculate 10 seconds to aggregate many indicators
	slot := int64((now+presetTtl)/10) * 10
	_ = uHandler.mList.Add(slot, indicator)
	return nil
}

func (wc *WarmCollector) invoke() {
	now := time.Now().Unix()
	slot := int64(now/10) * 10
	for instance, uHandler := range wc.handlers {
		sv, _ := uHandler.mList.Get(slot)
		if len(sv) == 0 {
			continue
		}
		for prefix, indicators := range sv {
			fn, bfn := uHandler.yf.getFunc(prefix)
			//opt use pool
			if bfn != nil {
				keys := make([]string, 0)
				for _, indicator := range indicators {
					_ = wc.lru.Delete(indicator.Id())
					keys = append(keys, indicator.Key())
				}
				ctx := context.WithValue(wc.newCtx(), "ycache", fmt.Sprintf("cache instance:%s warmCollector batchLoadFunc callback", instance))
				_ = uHandler.handler.BatchUpdate(ctx, prefix, keys, bfn)
			} else if fn != nil {
				for _, indicator := range indicators {
					ctx := context.WithValue(wc.newCtx(), "ycache", fmt.Sprintf("cache instance:%s warmCollector loadFunc callback", instance))
					_ = uHandler.handler.Update(ctx, prefix, indicator.Key(), fn)
					_ = wc.lru.Delete(indicator.Id())
				}
			}
			_ = uHandler.mList.Delete(slot)
		}
	}
}

type yFunc struct {
	mutex   sync.Mutex
	loadMap map[string]*yLoad
}

type yLoad struct {
	LoadFn      LoadFunc
	BatchLoadFn BatchLoadFunc
}

func (yf *yFunc) addFunc(prefix string, fn LoadFunc, bfn BatchLoadFunc) {
	yf.mutex.Lock()
	defer yf.mutex.Unlock()
	if load, ok := yf.loadMap[prefix]; ok {
		if load.LoadFn == nil {
			load.LoadFn = fn
		}
		if load.BatchLoadFn == nil {
			load.BatchLoadFn = bfn
		}
	} else {
		load = &yLoad{
			LoadFn:      fn,
			BatchLoadFn: bfn,
		}
		yf.loadMap[prefix] = load
	}
}

func (yf *yFunc) getFunc(prefix string) (fn LoadFunc, bfn BatchLoadFunc) {
	yf.mutex.Lock()
	defer yf.mutex.Unlock()
	if load, ok := yf.loadMap[prefix]; ok {
		return load.LoadFn, load.BatchLoadFn
	}
	return
}
