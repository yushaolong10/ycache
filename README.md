# ycache
level cache provides solution for high concurrent query scenarios.


### 需求背景 
面对业务中高频查询的需求，开发一款多级缓存框架

### 功能特点 
通过接口约束，支持个性化缓存组件 
通过回调方式优雅的支持单个，批次查询 
支持热点数据异步主动更新策略，业务无需读库 

### 设计方案
#### 1.多级缓存分层
```
L1 L2 L3 ...
interface Cache {
	Get(ctx, key string) ([]byte, error)
	BatchGet(ctx, keys []string) (map[string][]byte, error)
	Del(ctx, key string) error
	BatchDel(ctx, keys []string) error
}
```

#### 2.主动更新策略,2/3周期,采用时间轮进行更新,采用lru防止内存膨胀
```
interface Strategy {
	Add(ctx, key, fn)
	BatchAdd(ctx, keys, batchFn)
}
```

#### 3. 对外提供api

```
type LoadFunc func(ctx, string) ([]byte, error)

type BatchLoadFunc func(ctx, []string) (map[string][]byte, error)

type LevelCache struct {
	
}

func (lc *LevelCache) Get(ctx, prefix string, key string, loadFn LoadFunc) ([]byte, error) {
	
}

func (lc *LevelCache) BatchGet(ctx, prefix string, key []string, batchLoadFn BatchLoadFunc) (map[string][]byte, error) {
	
}
```


### 使用说明
```
//定义基础cache
cache := NewYCache(L1,L2)

//场景1:
//通用方式:
biz1 := cache.NewLevelCache(name1, {L1},strategy1, encode1)
value, err := biz1.Get(ctx,key, func(ctx, string) (interface{}, error) {
	
})

//场景2:
biz2 := cache.NewLevelCache(name2, {L1,L2}, strategy2, encode2)
//通用方式:
value, err := biz2.BatchGet(ctx,key, func(ctx, []string) (map[string]interface{}, error) {
	
})
```