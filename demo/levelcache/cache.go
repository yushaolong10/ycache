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
	for {
		time.Sleep(time.Second * 10)
		stat := Biz1Cache.Stat()
		data, _ := json.Marshal(stat)
		log.Printf("Biz1Cache stat:%s", string(data))
	}
}
