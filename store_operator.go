package goutils

/*
 data panel: 	local cache / remote cache(redis) / remote storage(mysql)
 control panel:	remote config / rolling
 feature: cb / limit / degrade

 本地缓存、分布式缓存、存储服务一体化高可用方案
 1. 熔断机制：当
 2. 报警：

 统计：rolling算法统计调用量和成功率，提供报警触发能力
 限流：根据通知对指定下游限流，可以配置redis/mysql两层限流
 熔断：根据成功率熔断，整体下游熔断，单个实例熔断由其他存储client实现
 多层降级：1. redis抖动或者宕机，开启local cache + 限流mysql模式 2. mysql宕机，降级到redis
 并发控制：single flight控制对下游并发，非强一致性
 热点数据读：远程配置热点数据，热点数据优先走本地缓存，然后依次读分布式缓存和存储，最终一致性。不负责热点发现，由行为日志捕捉

 其他：
 1. 统一大key拆分方案
 2.
*/

import (
	"golang.org/x/sync/singleflight"
)

type Config struct {
}

func (c *Config) IsHotKeyEnable(uniqKey string) bool {
	return true
}

func (c *Config) IsRedisAutoDegradeEnable() bool {
	return true
}

type Storage struct {
	singleflight.Group
}

func (st *Storage) Get(uniqKey string, param ...interface{}) (interface{}, bool) {
	return nil, true
}

func (st *Storage) IsFailOver() bool {
	return true
}


type Store struct {
	localCache  Storage
	remoteCache Storage
	storage     Storage
	config      Config
}

type ReqOption struct {
	ForceDb bool
}

func (s *Store) Get(uniqKey string, opt ReqOption, param ...interface{}) (interface{}, bool) {
	// 强制db
	if opt.ForceDb {
		return s.storage.Get(uniqKey, param...)
	}

	// 默认关闭local cache，可在remote config中指定
	localCacheEnable, remoteCacheEnable, remoteStorageEnable := false, true, true

	// 热点走local cache + redis + mysql
	if s.config.IsHotKeyEnable(uniqKey) {
		localCacheEnable = true
	}

	// redis抖动自动降级到local cache，类似熔断处理，redis内部探活
	if s.config.IsRedisAutoDegradeEnable() && s.localCache.IsFailOver() {
		localCacheEnable = true
		remoteCacheEnable = false
	}

	if localCacheEnable {
		ret, ok := s.localCache.Get(uniqKey, param...)
		if ok {
			return ret, ok
		}
	}
	if remoteCacheEnable {
		ret, ok := s.remoteCache.Get(uniqKey, param...)
		if ok {
			return ret, ok
		}
	}
	if remoteStorageEnable {
		ret, ok := s.storage.Get(uniqKey, param...)
		return ret, ok
	}
	return nil, false
}
