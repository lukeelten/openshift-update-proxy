package metrics

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"time"
)

type UpdateProxyMetrics struct {
	updateInfo   *prometheus.CounterVec
	healthchecks *prometheus.CounterVec
	cacheMiss    *prometheus.CounterVec
	cacheHit     *prometheus.CounterVec
	cacheEntries prometheus.Gauge

	upstreamResponseTime       *prometheus.HistogramVec
	cacheGarbageCollectionTime prometheus.Histogram

	Config *config.UpdateProxyConfig
	Server http.Server
}

func NewUpdateProxyMetrics(cfg *config.UpdateProxyConfig) *UpdateProxyMetrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &UpdateProxyMetrics{
		updateInfo:                 promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_cluster_infos"}, []string{"arch", "channel", "version"}),
		cacheMiss:                  promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_cache_miss"}, []string{"arch", "channel", "version"}),
		cacheHit:                   promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_cache_hit"}, []string{"arch", "channel", "version"}),
		upstreamResponseTime:       promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "openshift_update_proxy_upstream_response_time_ms"}, []string{"arch", "channel", "version"}),
		healthchecks:               promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_healthchecks"}, []string{"status"}),
		cacheEntries:               promauto.NewGauge(prometheus.GaugeOpts{Name: "openshift_update_proxy_cache_entries"}),
		cacheGarbageCollectionTime: promauto.NewHistogram(prometheus.HistogramOpts{Name: "openshift_update_proxy_cache_garbage_collection_time_ms"}),
		Server: http.Server{
			Handler: mux,
			Addr:    cfg.Metrics.Listen,
		},
	}
}

func (metrics *UpdateProxyMetrics) CacheGarbageCollectionTime(runtime time.Duration) {
	metrics.cacheGarbageCollectionTime.Observe(float64(runtime.Milliseconds()))
}

func (metrics *UpdateProxyMetrics) UpstreamResponseTime(arch, channel, version string, runtime time.Duration) {
	metrics.upstreamResponseTime.WithLabelValues(arch, channel, version).Observe(float64(runtime.Milliseconds()))
}

func (metrics *UpdateProxyMetrics) CacheHit(arch, channel, version string) {
	metrics.cacheHit.WithLabelValues(arch, channel, version).Inc()
}

func (metrics *UpdateProxyMetrics) CacheMiss(arch, channel, version string) {
	metrics.cacheMiss.WithLabelValues(arch, channel, version).Inc()
}

func (metrics *UpdateProxyMetrics) CacheEntries(length int) {
	metrics.cacheEntries.Set(float64(length))
}

func (metrics *UpdateProxyMetrics) UpdateInfo(arch, channel, version string) {
	metrics.updateInfo.WithLabelValues(arch, channel, version).Inc()
}

func (metrics *UpdateProxyMetrics) Healthcheck() {
	metrics.healthchecks.WithLabelValues(strconv.FormatBool(true)).Inc()
}

func (metrics *UpdateProxyMetrics) Run() error {
	return metrics.Server.ListenAndServe()
}

func (metrics *UpdateProxyMetrics) Shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return metrics.Server.Shutdown(shutdownCtx)
}
