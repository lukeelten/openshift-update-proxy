package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type HealthCheckCache struct {
	Status atomic.Bool
	lock   sync.Mutex

	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Client http.Client
}

func NewHealthCheckCache(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger) *HealthCheckCache {
	hCache := &HealthCheckCache{
		Status: atomic.Bool{},
		lock:   sync.Mutex{},
		Config: cfg,
		Logger: logger,
		Client: http.Client{
			Timeout: cfg.Health.Timeout,
		},
	}

	if cfg.Health.Insecure {
		tlsConfig := tls.Config{
			InsecureSkipVerify: true,
		}
		transport := http.Transport{
			TLSClientConfig: &tlsConfig,
		}
		hCache.Client.Transport = &transport
	}

	return hCache
}

func (cache *HealthCheckCache) Alive() bool {
	return cache.Status.Load()
}

func (cache *HealthCheckCache) refresh() error {
	cache.lock.Lock()
	defer cache.lock.Unlock()

	url := cache.Config.Health.Url
	if len(url) == 0 {
		url = cache.Config.UpstreamUrl
	}

	res, err := cache.Client.Get(url)
	if err != nil {
		cache.Logger.Debugw("got invalid response", "err", err, "response", res)
		return err
	}

	if res.StatusCode >= 400 {
		cache.Logger.Debugw("got invalid response", "response", res)
		return errors.New("got invalid status code")
	}

	return nil
}

func (cache *HealthCheckCache) Run(ctx context.Context) error {
	cache.Logger.Debug("Starting HealthCheck loop")
	cache.Status.Store(true)

	for {
		select {
		case <-time.NewTimer(cache.Config.Health.Interval).C:

			attempts := 0
			for {
				err := cache.refresh()
				if err == nil {
					break
				}

				cache.Logger.Infow("failing healthcheck", "err", err)

				if attempts >= cache.Config.Health.FailureThreshold {
					cache.Status.Store(false)
				}

				select {
				case <-ctx.Done():
					return nil
				case <-time.NewTimer(cache.Config.Health.RetryInterval).C:
					attempts += 1
				}
			}

			cache.Logger.Info("healthcheck update successful")
			cache.Status.Store(true)
			continue

		case <-ctx.Done():
			return nil
		}
	}
}
