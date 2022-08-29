package ycache

import (
	"context"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"time"
)

type RedisConfig struct {
	MaxTry         int
	WriteAddr      string
	ReadAddr       string
	Password       string
	ReadTimeout    int
	WriteTimeout   int
	ConnTimeout    int
	MaxActiveConns int
	MaxIdleConns   int
	IdleTimeout    int
}

func NewRedisClient(name string, conf *RedisConfig) *RedisClient {
	opts := []redis.DialOption{
		redis.DialPassword(conf.Password),
		redis.DialConnectTimeout(time.Duration(conf.ConnTimeout) * time.Second),
		redis.DialWriteTimeout(time.Duration(conf.WriteTimeout) * time.Second),
		redis.DialReadTimeout(time.Duration(conf.ReadTimeout) * time.Second),
	}
	readPool := &redis.Pool{
		MaxActive:   conf.MaxActiveConns,
		MaxIdle:     conf.MaxIdleConns,
		IdleTimeout: time.Duration(conf.IdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", conf.ReadAddr, opts...)
			if err != nil {
				return nil, fmt.Errorf("redis read addr dial err:%s", err.Error())
			}
			return conn, nil
		},
	}
	writePool := &redis.Pool{
		MaxActive:   conf.MaxActiveConns,
		MaxIdle:     conf.MaxIdleConns,
		IdleTimeout: time.Duration(conf.IdleTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", conf.WriteAddr, opts...)
			if err != nil {
				return nil, fmt.Errorf("redis write addr dial err:%s", err.Error())
			}
			return conn, nil
		},
	}
	client := &RedisClient{
		name:      name,
		maxTry:    conf.MaxTry,
		readPool:  readPool,
		writePool: writePool,
	}
	return client
}

type RedisClient struct {
	name      string
	maxTry    int
	readPool  *redis.Pool
	writePool *redis.Pool
}

func (client *RedisClient) Name() string {
	return client.name
}

//Get for get key value
func (client *RedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := redis.String(client.readDo("GET", key))
	if err != nil {
		return nil, fmt.Errorf("redisclient get key:%s err:%s", key, err.Error())
	}
	return []byte(value), nil
}

//BatchGet for batch get keys values
func (client *RedisClient) BatchGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	var keyList = make([]interface{}, 0)
	for _, key := range keys {
		keyList = append(keyList, key)
	}
	kvList, err := redis.Values(client.readDo("MGET", keyList...))
	if err != nil {
		return nil, fmt.Errorf("redisclient mget keys:%v err:%s", keys, err.Error())
	}
	var data = make(map[string][]byte)
	for index := 0; index < len(kvList); index = index + 2 {
		if len(kvList) >= index {
			break
		}
		if kvList[index+1] == nil {
			continue
		}
		key := kvList[index].([]byte)
		value := kvList[index+1].([]byte)
		data[string(key)] = value
	}
	return data, nil
}

//Del for delete a key
func (client *RedisClient) Del(ctx context.Context, key string) error {
	_, err := client.writeDo("DEL", key)
	if err != nil {
		return fmt.Errorf("redisclient del key(%s) err:%s", key, err.Error())
	}
	return nil
}

//BatchDel for batch delete keys
func (client *RedisClient) BatchDel(ctx context.Context, keys []string) error {
	var keyList = make([]interface{}, 0)
	for _, key := range keys {
		keyList = append(keyList, key)
	}
	_, err := client.writeDo("DEL", keyList...)
	if err != nil {
		return fmt.Errorf("redisclient batch del key err:%s", err.Error())
	}
	return nil
}

//Set for set key value with expire ttl seconds
func (client *RedisClient) Set(ctx context.Context, key string, value []byte, ttl int) error {
	_, err := client.writeDo("SETEX", key, ttl, string(value))
	if err != nil {
		return fmt.Errorf("redisclient set key(%s) err:%s", key, err.Error())
	}
	return nil
}

//BatchSet for set key value with expire ttl seconds
func (client *RedisClient) BatchSet(ctx context.Context, kvs map[string][]byte, ttl int) error {
	_, err := client.writePipeline(func(conn redis.Conn) error {
		for key, value := range kvs {
			err := conn.Send("SETEX", key, ttl, string(value))
			if err != nil {
				return fmt.Errorf("redisclient write pipeline set key(%s) err:%s", key, err.Error())
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("redisclient batch set err:%s", err.Error())
	}
	return nil
}

func (client *RedisClient) readDo(commandName string, args ...interface{}) (reply interface{}, err error) {
	conn := client.readPool.Get()
	defer func() {
		_ = conn.Close()
	}()
	return client.do(conn, commandName, args...)
}

func (client *RedisClient) writeDo(commandName string, args ...interface{}) (reply interface{}, err error) {
	conn := client.writePool.Get()
	defer func() {
		_ = conn.Close()
	}()
	return client.do(conn, commandName, args...)
}

func (client *RedisClient) writePipeline(pipeFn func(conn redis.Conn) error) (reply interface{}, err error) {
	conn := client.writePool.Get()
	defer func() {
		_ = conn.Close()
	}()
	return client.pipeline(conn, pipeFn)
}

func (client *RedisClient) pipeline(conn redis.Conn, pipeFn func(conn redis.Conn) error) (reply interface{}, err error) {
	err = conn.Send("MULTI")
	if err != nil {
		return nil, err
	}
	err = pipeFn(conn)
	if err != nil {
		return nil, err
	}
	return client.do(conn, "EXEC")
}

func (client *RedisClient) do(conn redis.Conn, commandName string, args ...interface{}) (reply interface{}, err error) {
	for i := 0; i < client.maxTry; i++ {
		reply, err = conn.Do(commandName, args...)
		if i == client.maxTry-1 || err == nil || err == redis.ErrNil {
			return reply, err
		}
	}
	return nil, fmt.Errorf("redisclient max try is 0")
}
