package metrics

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type UpdateProxyMetrics struct {
	Server http.Server

	MetricCacheHit  *prometheus.CounterVec
	MetricCacheMiss *prometheus.CounterVec
	CacheSize       *prometheus.GaugeVec
	VersionAccessed *prometheus.CounterVec
	Healthcheck     prometheus.Counter

	UpstreamResponseTime *prometheus.HistogramVec
	ResponseTime         *prometheus.HistogramVec
	ErrorResponses       *prometheus.CounterVec

	RefreshCounter *prometheus.CounterVec
	RefreshErrors  *prometheus.CounterVec
}

func NewUpdateProxyMetrics(cfg *config.UpdateProxyConfig) *UpdateProxyMetrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &UpdateProxyMetrics{
		MetricCacheMiss: promauto.NewCounterVec(utils.Counter("cache", "miss"), []string{"arch", "channel", "version"}),
		MetricCacheHit:  promauto.NewCounterVec(utils.Counter("cache", "hit"), []string{"arch", "channel", "version"}),
		CacheSize:       promauto.NewGaugeVec(utils.Gauge("cache", "size"), []string{"endpoint"}),
		VersionAccessed: promauto.NewCounterVec(utils.Counter("version", "access"), []string{"arch", "channel", "version"}),
		Healthcheck:     promauto.NewCounter(utils.Counter("healthcheck", "requests")),

		UpstreamResponseTime: promauto.NewHistogramVec(utils.Histogram("upstream", "response_time_ms"), []string{"arch", "channel", "version"}),
		ResponseTime:         promauto.NewHistogramVec(utils.Histogram("version", "response_time_ms"), []string{"endpoint"}),

		ErrorResponses: promauto.NewCounterVec(utils.Counter("response", "errors"), []string{"path"}),
		RefreshCounter: promauto.NewCounterVec(utils.Counter("version", "refreshed"), []string{"arch", "channel", "version"}),
		RefreshErrors:  promauto.NewCounterVec(utils.Counter("version", "refresh_errors"), []string{"arch", "channel", "version"}),

		Server: http.Server{
			Handler: mux,
			Addr:    cfg.Metrics.Listen,
		},
	}
}

func (metrics *UpdateProxyMetrics) Run() error {
	return metrics.Server.ListenAndServe()
}

func (metrics *UpdateProxyMetrics) Shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return metrics.Server.Shutdown(shutdownCtx)
}
