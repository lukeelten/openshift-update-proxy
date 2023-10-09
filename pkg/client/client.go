package client

import (
	"errors"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"time"
)

type OpenShiftVersionClient struct {
	logger   *zap.SugaredLogger
	config   *config.UpdateProxyConfig
	cache    *OpenShiftVersionCache
	upstream *UpstreamClient

	metrics *metrics.UpdateProxyMetrics
}

func NewOpenShiftVersionClient(cfg *config.UpdateProxyConfig, m *metrics.UpdateProxyMetrics, logger *zap.SugaredLogger, endpoint string, insecure bool, timeout time.Duration) *OpenShiftVersionClient {
	return &OpenShiftVersionClient{
		logger:   logger,
		config:   cfg,
		metrics:  m,
		cache:    NewOpenShiftVersionCache(cfg.Cache.DefaultLifetime, logger),
		upstream: NewUpstreamClient(logger, m, endpoint, insecure, timeout),
	}
}

func (client *OpenShiftVersionClient) CollectGarbage() {
	now := time.Now()
	num := 0

	client.cache.Foreach(func(entry VersionEntry) {
		if now.After(entry.LastAccessed.Add(client.config.Cache.EvictAfter)) {
			client.logger.Debugw("Delete entry from cache", "entry", entry)
			client.cache.Delete(entry.Arch, entry.Channel, entry.Version)
			num++
		}
	})

	if num > 0 {
		client.logger.Infow("Deleted entries from cache", "entries", num)
	}

	client.metrics.CacheSize.WithLabelValues(client.upstream.Endpoint).Set(client.cache.Size())
}

func (client *OpenShiftVersionClient) RefreshEntries() {
	now := time.Now()
	client.cache.Foreach(func(entry VersionEntry) {
		if now.After(entry.ValidUntil) {
			client.logger.Debugw("start refresh entry", "entry", entry)

			if client.loadFromUpstream(entry.Arch, entry.Channel, entry.Version) {
				client.metrics.RefreshCounter.WithLabelValues(entry.Arch, entry.Channel, entry.Version).Inc()
			} else {
				client.metrics.RefreshErrors.WithLabelValues(entry.Arch, entry.Channel, entry.Version).Inc()
				client.logger.Errorw("got error refreshing entry", "arch", entry.Arch, "channel", entry.Channel, "version", entry.Version)
			}
		}
	})
	client.metrics.CacheSize.WithLabelValues(client.upstream.Endpoint).Set(client.cache.Size())
}

func (client *OpenShiftVersionClient) Load(request *http.Request) ([]byte, error) {
	client.logger.Debugw("got request", "request", request)

	arch, channel, version := utils.ExtractQueryParams(request)
	if len(arch) == 0 || len(channel) == 0 || len(version) == 0 {
		client.logger.Errorw("cannot extract version information")
		return []byte{}, errors.New("no channel information present. Invalid request")
	}

	client.metrics.VersionAccessed.WithLabelValues(arch, channel, version).Inc()
	client.logger.Infow("got request for versions", "arch", arch, "channel", channel, "version", version)

	if !client.cache.HasKey(arch, channel, version) {
		client.metrics.MetricCacheMiss.WithLabelValues(arch, channel, version).Inc()
		if !client.loadFromUpstream(arch, channel, version) {
			client.logger.Errorw("cannot load version info from upstream")
			client.logger.Debugw("error on request", "request", request)
			return []byte{}, errors.New("no version info found")
		}
	} else {
		client.metrics.MetricCacheHit.WithLabelValues(arch, channel, version).Inc()
	}

	return client.cache.Get(arch, channel, version)
}

func (client *OpenShiftVersionClient) loadFromUpstream(arch, channel, version string) bool {
	client.logger.Infow("loading info from upstream", "arch", arch, "channel", channel, "version", version)
	versionBody, err := client.upstream.LoadVersionInfo(arch, channel, version)

	if err != nil {
		client.logger.Debugw("got error when loading upstream", "error", err, "arch", arch, "channel", channel, "version", version, "endpoint", client.upstream.Endpoint)
		client.logger.Errorw("error loading from upstream", "err", err)
		client.metrics.ErrorResponses.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()
		return false
	}

	client.cache.Set(arch, channel, version, versionBody)
	client.metrics.CacheSize.WithLabelValues(client.upstream.Endpoint).Set(client.cache.Size())
	return true
}
