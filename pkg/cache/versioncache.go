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
	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	product string
	Metrics *metrics.UpdateProxyMetrics

	lock  sync.Mutex
	cache map[string]*VersionEntry
}

func NewOpenShiftVersionCache(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, metric *metrics.UpdateProxyMetrics, product string) *OpenShiftVersionCache {
	metric.CacheSize.WithLabelValues(product).Set(0)
	return &OpenShiftVersionCache{
		Config:  cfg,
		Logger:  logger,
		product: product,
		Metrics: metric,

		lock:  sync.Mutex{},
		cache: make(map[string]*VersionEntry),
	}
}

func (cache *OpenShiftVersionCache) updateCacheMetric() {
	size := len(cache.cache)
	cache.Metrics.CacheSize.WithLabelValues(cache.product).Set(float64(size))
}

func (cache *OpenShiftVersionCache) Get(arch, channel, version string) ([]byte, error) {
	cache.updateCacheMetric()
	cache.Metrics.VersionAccessed.WithLabelValues(cache.product, arch, channel, version).Inc()

	key := makeKey(arch, channel, version)
	cache.Logger.Debugw("load cache entry", "key", key)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	entry, ok := cache.cache[key]
	if ok {
		cache.Metrics.MetricCacheHit.WithLabelValues(cache.product).Inc()
		entry.LastAccessed = time.Now()
		return entry.Body, nil
	}

	cache.Metrics.MetricCacheMiss.WithLabelValues(cache.product).Inc()
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

	cache.updateCacheMetric()
}

func (cache *OpenShiftVersionCache) Delete(arch, channel, version string) {
	key := makeKey(arch, channel, version)
	cache.lock.Lock()
	defer cache.lock.Unlock()

	delete(cache.cache, key)

	cache.updateCacheMetric()
}

func (cache *OpenShiftVersionCache) Foreach(foreachFunc ForeachFunc) {
	cache.updateCacheMetric()

	cache.lock.Lock()
	defer cache.lock.Unlock()

	for _, entry := range cache.cache {
		foreachFunc(entry)
	}
}

func (cache *OpenShiftVersionCache) DeleteAll(filterFunc FilterFunc) int {
	result := make([]string, 0)
	cache.lock.Lock()
	defer cache.lock.Unlock()

	for _, entry := range cache.cache {
		if filterFunc(entry) {
			result = append(result, entry.Key())
		}
	}

	for _, entry := range result {
		delete(cache.cache, entry)
	}

	cache.updateCacheMetric()
	return len(result)
}
