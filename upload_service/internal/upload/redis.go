package upload

import "github.com/go-redis/redis"

type RedisClient interface {
	WrapProcess(func(old func(cmd redis.Cmder) error) func(cmd redis.Cmder) error)
	HGet(key, field string) *redis.StringCmd
	LPush(key string, values ...interface{}) *redis.IntCmd
	RPush(key string, values ...interface{}) *redis.IntCmd
	HSet(key, field string, value interface{}) *redis.BoolCmd
}
