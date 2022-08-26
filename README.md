# ycache
level cache provides solution for high concurrent query scenarios.

(持续更新中...)
### 需求背景 
面对业务中高频查询的需求，开发一款多级缓存框架

### 功能特点 
- 通过接口约束，支持个性化缓存组件 
- 通过回调方式优雅的支持单个，批次查询
- 支持热点数据异步主动更新策略，业务无需读库 
- 支持防穿透配置,防止缓存穿透
- 防止并发请求落库
- 完善的监控指标

### 设计方案
#### 1.多级缓存分层
```
L1 L2 L3 ...
interface Cache {
	//Get for get key value
	Get(ctx context.Context, key string) ([]byte, error)
	//BatchGet for batch get keys values
	BatchGet(ctx context.Context, keys []string) (map[string][]byte, error)
	//Del for delete a key
	Del(ctx context.Context, key string) error
	//BatchDel for batch delete keys
	BatchDel(ctx context.Context, keys []string) error
	//Set for set key value with expire ttl seconds
	Set(ctx context.Context, key string, value []byte, ttl int) error
	//BatchSet for set key value with expire ttl seconds
	BatchSet(ctx context.Context, kvs map[string][]byte, ttl int) error
}
```

#### 2.主动更新策略,2/3周期,采用时间轮进行更新,采用lru防止内存膨胀
```
interface Strategy {
	//RegisterHandler  register cache instance
	RegisterHandler(name string, handler ICacheUpdateHandler, boundaryTtl int) error
	//UpdateIndicator for update indicator to trigger strategy
	UpdateIndicator(ctx context.Context, indicators []Indicator) error
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
biz1 := cache.CreateInstance(name1, {L1},strategy1)
value, err := biz1.Get(ctx,key, func(ctx, string) (interface{}, error) {
	
})

//场景2:
biz2 := cache.CreateInstance(name2, {L1,L2}, strategy2)
//通用方式:
value, err := biz2.BatchGet(ctx,key, func(ctx, []string) (map[string]interface{}, error) {
	
})
```