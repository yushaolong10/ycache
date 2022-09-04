package levelcache

import (
	"encoding/json"
	"fmt"
	"github.com/coocood/freecache"
	"github.com/gomodule/redigo/redis"
	"log"
	"strings"
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
	collectorConf := &ycache.WarmCollectorConfig{
		BuffSeconds: 10,
		EntryNumber: 100,
		TimeRatio:   60,
		MaxHotCount: 1000,
		HotKeyTtl:   60,
	}
	collector := ycache.NewWarmCollector(collectorConf)
	cache := ycache.NewYCache("my_first_test",
		ycache.WithCacheOptionErrorHandle(func(err error) {
			if !strings.Contains(err.Error(), freecache.ErrNotFound.Error()) &&
				!strings.Contains(err.Error(), redis.ErrNil.Error()) {
				fmt.Printf("ycache err:%s\n", err.Error())
			}
		}),
		ycache.WithCacheOptionCacheLevel(ycache.CacheL1,
			ycache.NewMemCache("my_first_mem_cache", 10000000, -1), //10M byte
		),
		ycache.WithCacheOptionCacheLevel(ycache.CacheL2,
			ycache.NewRedisClient("my_redis_cache", redisConf),
		),
	)
	var err error
	Biz1Cache, err = cache.CreateInstance("my_instance_1",
		[]ycache.CacheLevel{ycache.CacheL1, ycache.CacheL2},
		ycache.WithInstanceOptionCacheTtl(60),
		ycache.WithInstanceOptionRandomTtl(30),
		ycache.WithInstanceOptionTtlFactor(5),
		ycache.WithInstanceOptionCollector(collector),
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
			TotalUpdateCount:    stat.TotalUpdateCount - stat.TotalUpdateCount,
			TotalUpdateFailed:   stat.TotalUpdateFailed - stat.TotalUpdateFailed,
			TotalLoadConcurrent: stat.TotalLoadConcurrent - stat.TotalLoadConcurrent,
			TotalLoadCount:      stat.TotalLoadCount - stat.TotalLoadCount,
			TotalLoadFailed:     stat.TotalLoadFailed - stat.TotalLoadFailed,
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
		prometheus.UpdateLevelCache("biz1", "ycache", "total_req_count", incr.TotalReqCount)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_req_failed", incr.TotalReqFailed)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_update_count", incr.TotalUpdateCount)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_update_failed", incr.TotalUpdateFailed)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_load_concurrent", incr.TotalLoadConcurrent)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_load_count", incr.TotalLoadCount)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_load_failed", incr.TotalLoadFailed)
		prometheus.UpdateLevelCache("biz1", "ycache", "total_load_count", incr.TotalLoadCount)
		for level, cs := range incr.CacheStats {
			prometheus.UpdateLevelCache("biz1", level, "req_count", cs.ReqCount)
			prometheus.UpdateLevelCache("biz1", level, "req_failed", cs.ReqFailed)
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
