package levelcache

import (
	"encoding/json"
	"fmt"
	"github.com/coocood/freecache"
	"github.com/gomodule/redigo/redis"
	"log"
	"time"
	"ycache"
	"ycache/demo/prometheus"
)

var Biz1Cache *ycache.YInstance

func Init() error {
	var redisHost = "127.0.0.1:6379"
	var redisPwd = ""
	redisConf := &ycache.RedisConfig{
		MaxTry:         2,
		WriteAddr:      redisHost,
		ReadAddr:       redisHost,
		Password:       redisPwd,
		ReadTimeout:    1,
		WriteTimeout:   1,
		ConnTimeout:    3,
		MaxActiveConns: 64,
		MaxIdleConns:   5,
		IdleTimeout:    10,
	}
	cache := ycache.NewYCache("my_ycache_test",
		ycache.WithCacheOptionErrorHandle(func(desc string, err error) {
			if err != freecache.ErrNotFound && err != redis.ErrNil {
				fmt.Printf("ycache err:%s\n", err.Error())
			}
		}),
		ycache.WithCacheOptionCacheLevel(ycache.CacheL1,
			ycache.NewMemCache("my_first_mem_cache", 10000000, -1), //10M byte
		),
		ycache.WithCacheOptionCacheLevel(ycache.CacheL2,
			ycache.NewRedisClient("my_second_redis_cache", redisConf),
		),
	)
	var err error
	Biz1Cache, err = cache.CreateInstance("my_instance_1",
		[]ycache.CacheLevel{ycache.CacheL1, ycache.CacheL2},
		ycache.WithInstanceOptionCacheTtl(60),
		ycache.WithInstanceOptionRandomTtl(30),
		ycache.WithInstanceOptionTtlFactor(5),
	)
	if err != nil {
		return fmt.Errorf("create instance1 err:%s", err.Error())
	}
	go monitor()
	return nil
}

func monitor() {
	var i int64
	var oldStat = &ycache.YStat{CacheStats: make(map[string]*ycache.CacheStat)}
	for {
		time.Sleep(time.Second * 1)
		stat := Biz1Cache.Stat()
		incr := &ycache.YStat{
			TotalReqCount:       stat.TotalReqCount - oldStat.TotalReqCount,
			TotalReqFailed:      stat.TotalReqFailed - oldStat.TotalReqFailed,
			TotalLoadConcurrent: stat.TotalLoadConcurrent - oldStat.TotalLoadConcurrent,
			TotalLoadCount:      stat.TotalLoadCount - oldStat.TotalLoadCount,
			TotalLoadFailed:     stat.TotalLoadFailed - oldStat.TotalLoadFailed,
			CacheStats:          make(map[string]*ycache.CacheStat),
		}
		for name, cs := range stat.CacheStats {
			oldCs := &ycache.CacheStat{}
			if tmp, ok := oldStat.CacheStats[name]; ok {
				oldCs = tmp
			}
			incr.CacheStats[name] = &ycache.CacheStat{
				ReqCount:  cs.ReqCount - oldCs.ReqCount,
				ReqFailed: cs.ReqFailed - oldCs.ReqFailed,
			}
		}
		prometheus.UpdateLevelCache("ycache", Biz1Cache.Name(), "total_req_count", incr.TotalReqCount)
		prometheus.UpdateLevelCache("ycache", Biz1Cache.Name(), "total_req_failed", incr.TotalReqFailed)
		prometheus.UpdateLevelCache("ycache", Biz1Cache.Name(), "total_load_concurrent", incr.TotalLoadConcurrent)
		prometheus.UpdateLevelCache("ycache", Biz1Cache.Name(), "total_load_count", incr.TotalLoadCount)
		prometheus.UpdateLevelCache("ycache", Biz1Cache.Name(), "total_load_failed", incr.TotalLoadFailed)
		for level, cs := range incr.CacheStats {
			prometheus.UpdateLevelCache(Biz1Cache.Name(), level, "req_count", cs.ReqCount)
			prometheus.UpdateLevelCache(Biz1Cache.Name(), level, "req_failed", cs.ReqFailed)
		}
		data, _ := json.Marshal(stat)
		i++
		if i%10 == 0 {
			log.Printf("Biz1Cache stat:%s", string(data))
		}
		//replace
		oldStat = stat
	}
}
