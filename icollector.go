package ycache

import "context"

type ICollector interface {
	//RegisterHandler register callback handler to process cache update
	RegisterHandler(instance string, handler IUpdateHandler, boundaryTtl int) error
	//UpdateIndicators for collect indicators from cache instance.
	UpdateIndicators(ctx context.Context, instance string, indicators []Indicator) error
}

//IUpdateHandler for cache implement callback
type IUpdateHandler interface {
	//Update for invoke to update cache
	Update(ctx context.Context, prefix string, key string, loadFn LoadFunc) error
	//BatchUpdate for invoke to batch update cache
	BatchUpdate(ctx context.Context, prefix string, keys []string, batchLoadFn BatchLoadFunc) error
}

//Indicator describe metrics
type Indicator interface {
	Prefix() string
	Id() string
	Key() string
	LoadFunc() LoadFunc
	BatchLoadFunc() BatchLoadFunc
}

func newYIndicator(id, prefix, key string, loadFn LoadFunc, batchLoadFn BatchLoadFunc) Indicator {
	return &yIndicator{
		id:          id,
		prefix:      prefix,
		key:         key,
		loadFn:      loadFn,
		batchLoadFn: batchLoadFn,
	}
}

type yIndicator struct {
	id          string
	prefix      string
	key         string
	ttl         int64
	loadFn      LoadFunc
	batchLoadFn BatchLoadFunc
}

func (indicator *yIndicator) Id() string {
	return indicator.id
}

func (indicator *yIndicator) Prefix() string {
	return indicator.prefix
}

func (indicator *yIndicator) Key() string {
	return indicator.key
}

func (indicator *yIndicator) LoadFunc() LoadFunc {
	return indicator.loadFn
}

func (indicator *yIndicator) BatchLoadFunc() BatchLoadFunc {
	return indicator.batchLoadFn
}
