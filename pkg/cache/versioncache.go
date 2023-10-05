package cache

import (
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
	"sync"
	"time"
)

type FilterFunc func(entry *VersionEntry) bool
type ForeachFunc func(entry *VersionEntry)

type OpenShiftVersionCache struct {
	defaultLifetime time.Duration

	lock  sync.RWMutex
	cache map[string]VersionEntry
}

func NewOpenShiftVersionCache() *OpenShiftVersionCache {
	return &OpenShiftVersionCache{
		lock:  sync.RWMutex{},
		cache: make(map[string]VersionEntry),
	}
}

func (cache *OpenShiftVersionCache) Get(arch, channel, version string) ([]byte, error) {
	key := utils.MakeKey(arch, channel, version)

	cache.lock.RLock()
	defer cache.lock.RLock()

	entry, ok := cache.cache[key]
	if ok {
		return entry.Body, nil
	}

	return []byte{}, utils.ERR_NOT_FOUND
}

func (cache *OpenShiftVersionCache) Set(arch, channel, version string, body []byte) {
	entry := VersionEntry{
		Arch:    arch,
		Channel: channel,
		Version: version,
		Body:    body,
	}

	key := entry.Key()

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
	cacheCopy := cache.deepCopy()

	for _, entry := range cacheCopy {
		foreachFunc(&entry)
	}
}

func (cache *OpenShiftVersionCache) DeleteAll(filterFunc FilterFunc) int {
	cacheCopy := cache.deepCopy()

	cache.lock.Lock()
	defer cache.lock.Unlock()
	numDeleted := 0
	for _, entry := range cacheCopy {
		if filterFunc(&entry) {
			delete(cache.cache, entry.Key())
			numDeleted++
		}
	}

	return numDeleted
}

func (cache *OpenShiftVersionCache) deepCopy() map[string]VersionEntry {
	cache.lock.RLock()
	defer cache.lock.RLock()

	newMap := make(map[string]VersionEntry)
	for key, value := range cache.cache {
		newMap[key] = VersionEntry{
			Arch:    value.Arch,
			Channel: value.Channel,
			Version: value.Version,
			Body:    value.Body,
		}
	}

	return newMap
}

func (cache *OpenShiftVersionCache) Size() int {
	cache.lock.RLock()
	defer cache.lock.RUnlock()
	return len(cache.cache)
}
