package cache

import (
	"context"
	"fmt"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"go.uber.org/zap"
	"sync"
	"time"
)

type OpenShiftVersionCache struct {
	Config  *config.UpdateProxyConfig
	Logger  *zap.SugaredLogger
	Metrics *metrics.UpdateProxyMetrics

	cacheMap map[string]VersionCacheEntry
	lock     sync.RWMutex
}

type VersionCacheEntry struct {
	Body       []byte
	ValidUntil time.Time
}

type VersionKey struct {
	Version string
	Channel string
	Arch    string
}

func (key VersionKey) String() string {
	return fmt.Sprintf("%s%s%s", key.Arch, key.Channel, key.Version)
}

func (entry VersionCacheEntry) IsValid() bool {
	return len(entry.Body) > 0 && time.Now().Before(entry.ValidUntil)
}

func NewOpenShiftVersionCache(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, metric *metrics.UpdateProxyMetrics) *OpenShiftVersionCache {
	metric.CacheEntries(0)

	return &OpenShiftVersionCache{
		Config:   cfg,
		Logger:   logger,
		Metrics:  metric,
		cacheMap: make(map[string]VersionCacheEntry),
		lock:     sync.RWMutex{},
	}
}

func (cache *OpenShiftVersionCache) RunGarbageCollector(ctx context.Context) error {
	for {
		cache.Logger.Debug("Cache garbage collection")

		startTime := time.Now()
		cache.garbageCollection()
		runtime := time.Since(startTime)
		cache.Metrics.CacheGarbageCollectionTime(runtime)
		cache.updateMetrics()

		select {
		case <-ctx.Done():
			return nil
		case <-time.NewTimer(1 * time.Minute).C:
			continue
		}
	}
}

func (cache *OpenShiftVersionCache) garbageCollection() {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	deleteKeys := make([]string, 0)
	for key, entry := range cache.cacheMap {
		if !entry.IsValid() {
			deleteKeys = append(deleteKeys, key)
		}
	}

	for _, key := range deleteKeys {
		hashedKey := hash(key)
		delete(cache.cacheMap, hashedKey)
	}
}

func (cache *OpenShiftVersionCache) GetOrCompute(key VersionKey, computeFunc func() ([]byte, error)) ([]byte, error) {
	entry, err := cache.Get(key.String())
	if err == nil {
		cache.Metrics.CacheHit(key.Arch, key.Channel, key.Version)
		cache.Logger.Debugw("cache hit", "version", key)
		return entry, nil
	}

	cache.Metrics.CacheMiss(key.Arch, key.Channel, key.Version)
	cache.Logger.Debugw("cache miss", "version", key)

	body, err := computeFunc()
	if err != nil {
		return []byte{}, err
	}

	cache.Set(key.String(), body)
	return body, nil
}

func (cache *OpenShiftVersionCache) Set(key string, body []byte) {
	hashedKey := hash(key)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.cacheMap[hashedKey] = VersionCacheEntry{
		Body:       body,
		ValidUntil: time.Now().Add(cache.Config.DefaultTTL),
	}

	cache.updateMetrics()
}

func (cache *OpenShiftVersionCache) Get(key string) ([]byte, error) {
	hashedKey := hash(key)

	cache.lock.RLock()
	entry, ok := cache.cacheMap[hashedKey]
	cache.lock.RUnlock()

	if !ok {
		return []byte{}, ERR_NOT_FOUND
	}

	if !entry.IsValid() {
		return []byte{}, ERR_EXPIRED
	}

	return entry.Body, nil
}

func (cache *OpenShiftVersionCache) updateMetrics() {
	cache.Metrics.CacheEntries(len(cache.cacheMap))
}
