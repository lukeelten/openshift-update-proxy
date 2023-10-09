package client

import (
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
	"go.uber.org/zap"
	"sync"
	"time"
)

type VersionEntry struct {
	Arch    string
	Channel string
	Version string

	Body []byte

	LastAccessed time.Time
	ValidUntil   time.Time
}

type ForeachFunc func(entry VersionEntry)

type OpenShiftVersionCache struct {
	Logger          *zap.SugaredLogger
	defaultLifetime time.Duration

	lock  sync.RWMutex
	cache map[string]VersionEntry
}

func NewOpenShiftVersionCache(defaultLifetime time.Duration, logger *zap.SugaredLogger) *OpenShiftVersionCache {
	return &OpenShiftVersionCache{
		Logger:          logger,
		defaultLifetime: defaultLifetime,

		lock:  sync.RWMutex{},
		cache: make(map[string]VersionEntry),
	}
}

func (cache *OpenShiftVersionCache) Size() float64 {
	return float64(len(cache.cache))
}

func (cache *OpenShiftVersionCache) HasKey(arch, channel, version string) bool {
	key := utils.MakeKey(arch, channel, version)

	cache.lock.RLock()
	defer cache.lock.RUnlock()

	_, hasKey := cache.cache[key]
	return hasKey
}

func (cache *OpenShiftVersionCache) Get(arch, channel, version string) ([]byte, error) {
	key := utils.MakeKey(arch, channel, version)
	cache.Logger.Debugw("load cache entry", "key", key)

	cache.lock.RLock()
	defer cache.lock.RUnlock()

	entry, ok := cache.cache[key]
	if ok {
		entry.LastAccessed = time.Now()
		return entry.Body, nil
	}

	return []byte{}, utils.ERR_NOT_FOUND
}

func (cache *OpenShiftVersionCache) Set(arch, channel, version string, body []byte) {
	entry := VersionEntry{
		Arch:         arch,
		Channel:      channel,
		Version:      version,
		Body:         body,
		LastAccessed: time.Now(),
		ValidUntil:   time.Now().Add(cache.defaultLifetime),
	}

	key := utils.MakeKey(entry.Arch, entry.Channel, entry.Version)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.cache[key] = entry
}

func (cache *OpenShiftVersionCache) Delete(arch, channel, version string) {
	key := utils.MakeKey(arch, channel, version)
	cache.lock.Lock()
	defer cache.lock.Unlock()

	delete(cache.cache, key)
}

func (cache *OpenShiftVersionCache) Foreach(foreachFunc ForeachFunc) {
	c := cache.lightCopy()

	for _, entry := range c {
		foreachFunc(entry)
	}
}

func (cache *OpenShiftVersionCache) lightCopy() map[string]VersionEntry {
	cache.lock.RLock()
	defer cache.lock.RUnlock()

	c := make(map[string]VersionEntry, len(cache.cache))

	for key, entry := range cache.cache {
		c[key] = VersionEntry{
			Arch:         entry.Arch,
			Channel:      entry.Channel,
			Version:      entry.Version,
			Body:         nil,
			LastAccessed: entry.LastAccessed,
			ValidUntil:   entry.ValidUntil,
		}
	}

	return c
}
