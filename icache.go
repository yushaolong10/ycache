package ycache

import "context"

type ICache interface {
	//Name for cache name
	Name() string
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
