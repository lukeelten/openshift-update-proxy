package cache

import (
	"sync"
	"time"
)

type ResponseCache struct {
	defaultTTL time.Duration
	cacheMap   map[string]ResponseCacheEntry
	lock       sync.RWMutex
}

type ResponseCacheEntry struct {
	Body       []byte
	ValidUntil time.Time
}

func NewResponseCache(defaultTTL time.Duration) *ResponseCache {
	return &ResponseCache{
		defaultTTL: defaultTTL,
		cacheMap:   make(map[string]ResponseCacheEntry),
		lock:       sync.RWMutex{},
	}
}

func (cache *ResponseCache) GetOrCompute(key string, computeFunc func() ([]byte, error)) ([]byte, error) {
	entry, err := cache.Get(key)
	if err == nil {
		return entry, nil
	}

	body, err := computeFunc()
	if err != nil {
		return []byte{}, err
	}

	cache.Set(key, body)
	return body, nil
}

func (cache *ResponseCache) Set(key string, body []byte) {
	hashedKey := hash(key)

	cache.lock.Lock()
	defer cache.lock.Unlock()

	cache.cacheMap[hashedKey] = ResponseCacheEntry{
		Body:       body,
		ValidUntil: time.Now().Add(cache.defaultTTL),
	}
}

func (cache *ResponseCache) Get(key string) ([]byte, error) {
	hashedKey := hash(key)
	now := time.Now()

	cache.lock.RLock()
	entry, ok := cache.cacheMap[hashedKey]
	cache.lock.RUnlock()

	if !ok {
		return []byte{}, ERR_NOT_FOUND
	}

	if now.After(entry.ValidUntil) {
		cache.lock.Lock()
		defer cache.lock.Unlock()
		newEntry, ok := cache.cacheMap[hashedKey]
		if !ok {
			return []byte{}, ERR_EXPIRED
		}

		if newEntry.ValidUntil.Equal(entry.ValidUntil) || newEntry.ValidUntil.Before(now) {
			delete(cache.cacheMap, hashedKey)
			return []byte{}, ERR_EXPIRED
		}

		// entry has been replaced since
		return newEntry.Body, nil
	}

	return entry.Body, nil
}
