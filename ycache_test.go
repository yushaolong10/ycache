package ycache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

var redisHost = "127.0.0.1:6379"
var redisPwd = "123456"
var cache *YCache
var bizCacheIns *YInstance

func TestMain(m *testing.M) {
	redisConf := &RedisConfig{
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
	collectorConf := &WarmCollectorConfig{
		BuffSeconds: 10,
		EntryNumber: 20,
		TimeRatio:   30,
		MaxHotCount: 1000,
		HotKeyTtl:   60,
	}
	collector := NewWarmCollector(collectorConf)
	cache = NewYCache("my_first_test",
		WithCacheOptionErrorHandle(func(err error) {
			//fmt.Printf("ycache err:%s\n", err.Error())
		}),
		WithCacheOptionCacheLevel(CacheL1,
			NewMemCache("my_first_mem_cache", 10000000, -1),
		),
		WithCacheOptionCacheLevel(CacheL2,
			NewRedisClient("my_redis_cache", redisConf),
		),
	)
	var err error
	bizCacheIns, err = cache.CreateInstance("my_instance_1",
		[]CacheLevel{CacheL1, CacheL2},
		WithInstanceOptionCacheTtl(60),
		WithInstanceOptionRandomTtl(20),
		WithInstanceOptionTtlFactor(1),
		WithInstanceOptionCollector(collector),
	)
	if err != nil {
		panic(fmt.Sprintf("create instance1 err:%s", err.Error()))
	}
	m.Run()
}

func TestYInstanceGetSet(t *testing.T) {
	key := "k1"
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("instance get err:%s", err.Error())
	}
	if string(val) != "hello" {
		t.Fatalf("instance get key must equal 'hello'")
	}
	time.Sleep(time.Second * 1)
	val, _ = bizCacheIns.Get(context.Background(), "_abc_", key, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("second"), nil
	})
	if string(val) == "second" {
		t.Fatalf("instance get key must not equal 'second'")
	}
	val, err = bizCacheIns.Get(context.Background(), "_abcd_", key, func(ctx context.Context, key string) ([]byte, error) {
		return nil, fmt.Errorf("prefix _abcd_ err")
	})
	if err != nil {
		t.Logf("instance get key load should err, key:%s,err:%s", key, err.Error())
	} else {
		t.Fatalf("instance get key must err")
	}
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceBatchGetSet(t *testing.T) {
	key1 := "k1"
	key2 := "k2"
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("instance get key1 err:%s", err.Error())
	}
	time.Sleep(time.Second * 2)
	keys := []string{key1, key2}
	mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", keys, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		ret := map[string][]byte{
			"k2": []byte("world"),
		}
		return ret, nil
	})
	for k, v := range mp {
		if k == key1 && string(v) != "hello" {
			t.Fatalf("batch get key1(%s) not equal", k)
		} else if k == key2 && string(v) != "world" {
			t.Fatalf("batch get key2(%s) not equal", k)
		}
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("world2"), nil
	})
	if err != nil {
		t.Fatalf("instance get key2 load err, key2:%s,err:%s", key2, err.Error())
	} else if string(val) == "world2" {
		t.Fatalf("instance get key2 not expired, must not equal 'world2'")
	}
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceBatchDel(t *testing.T) {
	key1 := "k1"
	key2 := "k2"
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("instance get key1 err:%s", err.Error())
	}
	time.Sleep(time.Second * 2)
	keys := []string{key1, key2}
	mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", keys, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		ret := map[string][]byte{
			"k2": []byte("world"),
		}
		return ret, nil
	})
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Fatalf("instance empty get key2 load err:%s", err.Error())
	} else if string(mp[key2]) != "world" {
		t.Fatalf("instance empty get key2 must equal 'world'")
	}
	err = bizCacheIns.Delete(context.Background(), "_abc_", key2)
	if err != nil {
		t.Fatalf("instance delete key2 err:%s", err.Error())
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Logf("instance get key2 load must err is normal")
	} else {
		t.Fatalf("instance get key2 must err")
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("new key2 val"), nil
	})
	if err != nil {
		t.Fatalf("instance get key2 load error, key:%s,err:%s", key2, err.Error())
	} else if string(val) != "new key2 val" {
		t.Fatalf("instance get key2 success, key:%s must equal 'new key2 val'", key2)
	}
	err = bizCacheIns.BatchDelete(context.Background(), "_abc_", []string{key2})
	if err != nil {
		t.Fatalf("instance batch delete key2 error, key:%s,err:%s", key2, err.Error())
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Logf("instance get key2 error is normal, key:%s,err:%s", key2, err.Error())
	} else {
		t.Fatalf("instance get key must error, key:%s", key2)
	}
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceStrategy(t *testing.T) {
	key1 := "k1"
	for i := 0; i < 20; i++ {
		_, err := bizCacheIns.Get(context.Background(), "_def_", key1, func(ctx context.Context, key string) ([]byte, error) {
			if i < 10 {
				return []byte("k1 hello"), nil
			} else {
				return []byte("k1 strategy ok"), nil
			}
		})
		if err != nil {
			t.Fatalf("instance get key1 error, index:%d,key:%s,err:%s", i, key1, err.Error())
		}
	}
	time.Sleep(time.Second * 3)
	for i := 0; i < 20; i++ {
		key2 := "k2"
		_, err := bizCacheIns.BatchGet(context.Background(), "_def_", []string{key2}, func(ctx context.Context, nKeys []string) (map[string][]byte, error) {
			ret := make(map[string][]byte)
			for _, key := range nKeys {
				if key == key1 {
					ret[key] = []byte("batch new k1")
				} else if key == key2 {
					ret[key] = []byte("batch new k2")
				}
			}
			return ret, nil
		})
		if err != nil {
			t.Fatalf("instance batch get key2 error, index:%d,key:%s,err:%s", i, key1, err.Error())
		}
	}
	time.Sleep(time.Second * 50)
	val, err := bizCacheIns.Get(context.Background(), "_def_", key1, nil)
	if err != nil {
		t.Fatalf("instance get key1 error, key:%s,err:%s", key1, err.Error())
	} else if string(val) != "batch new k1" {
		t.Fatalf("instance get key1 by strategy must equal 'batch new k1', key:%s,val:%s", key1, string(val))
	}
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceConcurrent(t *testing.T) {
	key1 := "k15"
	oldStat := bizCacheIns.Stat()

	for i := 0; i < 10; i++ {
		i := i
		go func() {
			_, _ = bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
				time.Sleep(time.Second)
				if i < 10 {
					return []byte("k1 hello"), nil
				} else {
					return []byte("k1 strategy ok"), nil
				}
			})
			//t.Logf("instance get key1 success, index:%d,val:%v", i, string(val))
		}()
	}
	time.Sleep(time.Second * 3)
	stat := bizCacheIns.Stat()
	if stat.TotalLoadCount-oldStat.TotalLoadCount != 1 {
		t.Fatalf("must load once")
	}

	key2 := "k25"
	for i := 0; i < 5; i++ {
		//i := i
		go func() {
			kvs, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", []string{key2, key1}, func(ctx context.Context, keys []string) (map[string][]byte, error) {
				time.Sleep(time.Second)
				return map[string][]byte{
					key1: []byte("k1 batch"),
					key2: []byte("k2 batch"),
				}, nil
			})
			mp := make(map[string]string)
			for k, v := range kvs {
				mp[k] = string(v)
			}
			//t.Logf("instance batch get key1,key2 success, index:%d,kvs:%v", i, mp)
		}()
	}
	time.Sleep(time.Second * 3)
	stat = bizCacheIns.Stat()
	if stat.TotalLoadCount-oldStat.TotalLoadCount != 2 {
		t.Fatalf("must load twice")
	}
}
