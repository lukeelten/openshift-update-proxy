package cache

import (
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"go.uber.org/zap"
	"sync"
	"time"
)

type FilterFunc func(entry *VersionEntry) bool
type ForeachFunc func(entry *VersionEntry)

type OpenShiftVersionCache struct {
	Config  *config.UpdateProxyConfig
	Logger  *zap.SugaredLogger
	Metrics *metrics.UpdateProxyMetrics

	lock  sync.Mutex
	cache map[string]*VersionEntry
}

func NewOpenShiftVersionCache(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, metric *metrics.UpdateProxyMetrics) *OpenShiftVersionCache {
	return &OpenShiftVersionCache{
		Config:  cfg,
		Logger:  logger,
		Metrics: metric,
		lock:    sync.Mutex{},
		cache:   make(map[string]*VersionEntry),
	}
}

func (cache *OpenShiftVersionCache) Get(arch, channel, version string) ([]byte, error) {
	key := makeKey(arch, channel, version)
	cache.Logger.Debugw("load cache entry", "key", key)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	entry, ok := cache.cache[key]
	if ok {
		// Todo: log access
		entry.LastAccessed = time.Now()
		return entry.Body, nil
	}

	return []byte{}, ERR_NOT_FOUND
}

func (cache *OpenShiftVersionCache) Set(arch, channel, version string, body []byte) {
	entry := VersionEntry{
		Arch:         arch,
		Channel:      channel,
		Version:      version,
		Body:         body,
		LastAccessed: time.Now(),
		ValidUntil:   time.Now().Add(cache.Config.Cache.DefaultLifetime),
	}

	key := entry.Key()

	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.cache[key] = &entry
}

func (cache *OpenShiftVersionCache) Delete(arch, channel, version string) {
	key := makeKey(arch, channel, version)
	cache.lock.Lock()
	defer cache.lock.Unlock()

	delete(cache.cache, key)
}

func (cache *OpenShiftVersionCache) Foreach(foreachFunc ForeachFunc) {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	for _, entry := range cache.cache {
		foreachFunc(entry)
	}
}

func (cache *OpenShiftVersionCache) DeleteAll(filterFunc FilterFunc) int {
	result := make([]*VersionEntry, 0)
	cache.lock.Lock()
	defer cache.lock.Unlock()

	for _, entry := range cache.cache {
		if filterFunc(entry) {
			result = append(result, entry)
		}
	}

	for _, entry := range result {
		delete(cache.cache, entry.Key())
	}

	return len(result)
}
