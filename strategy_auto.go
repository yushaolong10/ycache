package ycache

import (
	"context"
	"github.com/rfyiamcool/go-timewheel"
	"time"
)

type StrategyAutoConfig struct {
	BuffSeconds int64
	EntryNumber int64
	TimeRatio   int64
	MaxHotCount int64
	HotKeyTtl   int64
	NewContext  func() context.Context
}

func NewStrategyAuto(conf *StrategyAutoConfig) *StrategyAuto {
	//new time wheel
	tw, _ := timewheel.NewTimeWheel(1*time.Second, 3600)
	tw.Start()
	//create
	strategy := &StrategyAuto{
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
		strategy.newCtx = conf.NewContext
	}
	//invoke task
	tw.AddCron(time.Second*10, strategy.invoke)
	return strategy
}

type StrategyAuto struct {
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
	mList       *simpleMapList
}

func (sa *StrategyAuto) RegisterHandler(name string, handler IUpdateHandler, boundaryTtl int) error {
	sa.handlers[name] = &updateHandler{
		handler:     handler,
		boundaryTtl: int64(boundaryTtl),
		mList:       newSimpleMapList(),
	}
	return nil
}

func (sa *StrategyAuto) UpdateIndicators(ctx context.Context, indicators []Indicator) error {
	for _, indicator := range indicators {
		_ = sa.updateIndicator(ctx, indicator)
	}
	return nil
}

func (sa *StrategyAuto) updateIndicator(ctx context.Context, indicator Indicator) error {
	name := indicator.Name()
	if sa.handlers[name].boundaryTtl < sa.bufferSecs {
		return nil
	}
	number := sa.lru.Update(indicator.Id(), indicator)
	if number != sa.entryTimes {
		return nil
	}
	//in list no trigger again
	if sa.handlers[name].mList.Exist(indicator.Id()) {
		return nil
	}
	//here simply think current timestamp is the first create cache time
	//get preset ratio ttl
	presetTtl := int64(sa.handlers[name].boundaryTtl * sa.timeRatio / 100)
	//add to map list
	now := time.Now().Unix()
	//calculate 10 seconds to aggregate many indicators
	slot := int64((now+presetTtl)/10) * 10
	_ = sa.handlers[name].mList.Add(slot, indicator)
	return nil
}

func (sa *StrategyAuto) invoke() {
	now := time.Now().Unix()
	slot := int64(now/10) * 10
	for cacheName, uh := range sa.handlers {
		mvs, _ := uh.mList.Get(slot)
		if len(mvs) > 0 {
			for prefix, mv := range mvs {
				//opt use pool
				if mv.bfn != nil {
					keys := make([]string, 0)
					for _, indicator := range mv.indicators {
						_ = sa.lru.Delete(indicator.Id())
						keys = append(keys, indicator.Key())
					}
					ctx := context.WithValue(sa.newCtx(), "ycache", cacheName+"strategyAuto batch callback")
					_ = uh.handler.BatchUpdate(ctx, prefix, keys, mv.bfn)
				} else if mv.fn != nil {
					for _, indicator := range mv.indicators {
						ctx := context.WithValue(sa.newCtx(), "ycache", cacheName+"strategyAuto single callback")
						_ = uh.handler.Update(ctx, prefix, indicator.Key(), mv.fn)
						_ = sa.lru.Delete(indicator.Id())
					}
				}
			}
			_ = uh.mList.Delete(slot)
		}
	}
}
