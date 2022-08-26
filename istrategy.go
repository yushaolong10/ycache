package ycache

import "context"

type IStrategy interface {
	//RegisterHandler  register cache instance
	RegisterHandler(name string, handler IUpdateHandler, boundaryTtl int) error
	//UpdateIndicators for update indicator to trigger strategy
	UpdateIndicators(ctx context.Context, indicators []Indicator) error
}

type IUpdateHandler interface {
	//Update for invoke to update cache
	Update(ctx context.Context, prefix string, key string, loadFn LoadFunc) error
	//BatchUpdate for invoke to batch update cache
	BatchUpdate(ctx context.Context, prefix string, keys []string, batchLoadFn BatchLoadFunc) error
}

//Indicator metrics
type Indicator interface {
	Name() string
	Prefix() string
	Id() string
	Key() string
	LoadFunc() LoadFunc
	BatchLoadFunc() BatchLoadFunc
}

func newDefaultIndicator(name, id, prefix, key string, loadFn LoadFunc, batchLoadFn BatchLoadFunc) Indicator {
	return &yIndicator{
		name:        name,
		id:          id,
		prefix:      prefix,
		key:         key,
		loadFn:      loadFn,
		batchLoadFn: batchLoadFn,
	}
}

type yIndicator struct {
	name        string
	id          string
	prefix      string
	key         string
	ttl         int64
	loadFn      LoadFunc
	batchLoadFn BatchLoadFunc
}

func (indicator *yIndicator) Name() string {
	return indicator.name
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
