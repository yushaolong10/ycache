package ycache

import (
	"context"
	"encoding/json"
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
		EntryNumber: 2,
		TimeRatio:   30,
		MaxHotCount: 1000,
		HotKeyTtl:   60,
	}
	collector := NewWarmCollector(collectorConf)
	cache = NewYCache("my_first_test",
		WithCacheOptionErrorHandle(func(err error) {
			fmt.Printf("ycache err:%s\n", err.Error())
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
		WithInstanceOptionCacheTtl(10),
		WithInstanceOptionRandomTtl(20),
		WithInstanceOptionTtlFactor(5),
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
	t.Logf("instance get 1 success, key:%s,val:%s", key, string(val))
	time.Sleep(time.Second * 2)
	val, _ = bizCacheIns.Get(context.Background(), "_abc_", key, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("second"), nil
	})
	t.Logf("instance get 2 success, key:%s,val:%s", key, string(val))
	val, err = bizCacheIns.Get(context.Background(), "_abcd_", key, func(ctx context.Context, key string) ([]byte, error) {
		return nil, fmt.Errorf("second err")
	})
	if err != nil {
		t.Logf("instance get 3 load err, key:%s,err:%s", key, err.Error())
	} else {
		t.Logf("instance get 3 success, key:%s,val:%s", key, string(val))
	}
	stat, _ := json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceBatchGetSet(t *testing.T) {
	key1 := "k1"
	key2 := "k2"
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("instance get err:%s", err.Error())
	}
	t.Logf("instance get 1 success, key:%s,val:%s", key1, string(val))
	time.Sleep(time.Second * 2)
	keys := []string{key1, key2}
	mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", keys, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		ret := map[string][]byte{
			"k2": []byte("world"),
		}
		return ret, nil
	})
	t.Logf("instance get 2 success, keys:%v,vals:%s", keys, mp)
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("world"), nil
	})
	if err != nil {
		t.Logf("instance get 3 load err, key2:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance get 3 success, key2:%s,val:%s", key2, string(val))
	}
	stat, _ := json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceBatchDel(t *testing.T) {
	key1 := "k1"
	key2 := "k2"
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("hello"), nil
	})
	if err != nil {
		t.Fatalf("instance get err:%s", err.Error())
	}
	t.Logf("instance get 1 success, key:%s,val:%s", key1, string(val))
	time.Sleep(time.Second * 2)
	keys := []string{key1, key2}
	mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", keys, func(ctx context.Context, keys []string) (map[string][]byte, error) {
		ret := map[string][]byte{
			"k2": []byte("world"),
		}
		return ret, nil
	})
	t.Logf("instance get 2 success, keys:%v,vals:%s", keys, mp)
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Logf("instance empty get 3 load err, key:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance empty get 3 success, key:%s,val:%s", key2, string(val))
	}
	err = bizCacheIns.Delete(context.Background(), "_abc_", key2)
	if err != nil {
		t.Logf("instance delete 4 err, key2:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance delete 4 success, key2:%s", key2)
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Logf("instance get 5 load err, key:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance get 5 success, key:%s,val:%s", key2, string(val))
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, func(ctx context.Context, key string) ([]byte, error) {
		return []byte("new key2 val"), nil
	})
	if err != nil {
		t.Logf("instance get 6 load err, key:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance get 6 success, key:%s,val:%s", key2, string(val))
	}
	err = bizCacheIns.BatchDelete(context.Background(), "_abc_", []string{key2})
	if err != nil {
		t.Logf("instance get 7 load err, key:%s,err:%s", key2, err.Error())
	}
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key2, nil)
	if err != nil {
		t.Logf("instance get 8 load err, key:%s,err:%s", key2, err.Error())
	} else {
		t.Logf("instance get 8 success, key:%s,val:%s", key2, string(val))
	}
	stat, _ := json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceStrategy(t *testing.T) {
	key1 := "k1"
	for i := 0; i < 10; i++ {
		val, _ := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
			if i < 10 {
				return []byte("k1 hello"), nil
			} else {
				return []byte("k1 strategy ok"), nil
			}
		})
		t.Logf("instance get 1 success, index:%d,key:%s,val:%s", i, key1, string(val))
	}
	stat, _ := json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Second * 5)
	for i := 0; i < 10; i++ {
		key2 := "k2"
		mp, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", []string{key2}, func(ctx context.Context, nKeys []string) (map[string][]byte, error) {
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
		t.Logf("instance get 2 success, key2:%s,val:%s", key2, mp)
	}
	time.Sleep(time.Second * 30)
	val, err := bizCacheIns.Get(context.Background(), "_abc_", key1, nil)
	if err != nil {
		t.Logf("instance get 3 error, key:%s,err:%s", key1, err.Error())
	} else {
		t.Logf("instance get 3 success, key:%s,val:%s", key1, string(val))
	}
	time.Sleep(time.Second * 30)
	val, err = bizCacheIns.Get(context.Background(), "_abc_", key1, nil)
	if err != nil {
		t.Logf("instance get 4 error, key:%s,err:%s", key1, err.Error())
	} else {
		t.Logf("instance get 4 success, key:%s,val:%s", key1, string(val))
	}
	stat, _ = json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Millisecond * 50)
}

func TestYInstanceConcurrent(t *testing.T) {
	key1 := "k15"
	for i := 0; i < 20; i++ {
		i := i
		go func() {
			val, _ := bizCacheIns.Get(context.Background(), "_abc_", key1, func(ctx context.Context, key string) ([]byte, error) {
				time.Sleep(time.Second)
				fmt.Println("get in here", i)
				if i < 10 {
					return []byte("k1 hello"), nil
				} else {
					return []byte("k1 strategy ok"), nil
				}
			})
			t.Logf("instance get 1 success, index:%d,key:%s,val:%s", i, key1, string(val))
		}()
	}
	key2 := "k25"
	for i := 0; i < 5; i++ {
		i := i
		go func() {
			kvs, _ := bizCacheIns.BatchGet(context.Background(), "_abc_", []string{key2, key1}, func(ctx context.Context, keys []string) (map[string][]byte, error) {
				time.Sleep(time.Second)
				fmt.Println("batch in here ", i)
				return map[string][]byte{
					key1: []byte("k1 batch"),
					key2: []byte("k2 batch"),
				}, nil
			})
			mp := make(map[string]string)
			for k, v := range kvs {
				mp[k] = string(v)
			}
			t.Logf("instance get 2 success, index:%d,kvs:%v", i, mp)
		}()
	}
	stat, _ := json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Second * 10)
	stat, _ = json.Marshal(bizCacheIns.Stat())
	t.Logf("ins stat:%s", string(stat))
	time.Sleep(time.Second * 1)
}
