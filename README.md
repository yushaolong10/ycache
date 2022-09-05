# ycache

level cache provides solution for high concurrent query scenarios.

![Build Status](https://api.travis-ci.org/repos/yushaolong7/ycache?branch=master)
![GoCover](https://img.shields.io/badge/coverage-86.0%25-brightgreen.svg?style=flat)

## Features

- Supports personalized cache components through interface constraints
- Elegantly supports single and batch queries through callbacks
- Supports hot data asynchronous and active update strategy, business does not need to read from source
- Prevent concurrent requests from falling into the source
- Perfect monitoring indicators

## Architecture
### 1. Multi-level cache layering

<center>
    <img src="https://github.com/yushaolong7/ycache/blob/main/doc/arch-1-level.png" width=40% height=40% />
</center>

#### 2. Active update strategy extended

<center>
    <img src="https://github.com/yushaolong7/ycache/blob/main/doc/arch-2-collector.png" width=40% height=40% />
</center>

## Usage
```go
//1. create cache
cache = NewYCache("my_first_test",
	WithCacheOptionCacheLevel(CacheL1,
		NewMemCache("my_first_mem_cache", 10000000, -1),
	)
)
//2. create business cache instance
bizCacheIns, err = cache.CreateInstance("my_instance_1",
	[]CacheLevel{CacheL1, CacheL2},
	WithInstanceOptionCacheTtl(60),
)
//3. use instance 
//3.1 get
val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
		//todo your business
})
//3.2 batch get
keys := []string{key1, key2}
mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", keys, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		//todo your business
})
```
## Example

see [demo](https://github.com/yushaolong7/ycache/blob/main/demo),there are specific steps to run the demo application [here](https://github.com/yushaolong7/ycache/blob/main/demo/assets/usage) and grafana dashboard json model [config](https://github.com/yushaolong7/ycache/blob/main/demo/assets/prometheus.json). Eventually, the metrics in grafana show as:

<center>
    <img src="https://github.com/yushaolong7/ycache/blob/main/demo/assets/monitor.png">
</center>


## TODO

* Support collector quick strategy.

## License

The MIT License
