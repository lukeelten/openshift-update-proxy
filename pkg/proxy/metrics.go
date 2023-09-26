package proxy

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
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

const METRIC_NAMESPACE = "openshift_update_proxy"

func NewUpdateProxyMetrics(cfg *config.UpdateProxyConfig) *UpdateProxyMetrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &UpdateProxyMetrics{
		MetricCacheMiss: promauto.NewCounterVec(counter("cache", "miss"), []string{"product"}),
		MetricCacheHit:  promauto.NewCounterVec(counter("cache", "hit"), []string{"product"}),
		CacheSize:       promauto.NewGaugeVec(gauge("cache", "size"), []string{"product"}),
		VersionAccessed: promauto.NewCounterVec(counter("version", "access"), []string{"product", "arch", "channel", "version"}),
		Healthcheck:     promauto.NewCounter(counter("healthcheck", "requests")),

		UpstreamResponseTime: promauto.NewHistogramVec(histogram("upstream", "response_time_ms"), []string{"endpoint", "arch", "channel", "version"}),
		ResponseTime:         promauto.NewHistogramVec(histogram("version", "response_time_ms"), []string{"endpoint", "arch", "channel", "version"}),

		ErrorResponses: promauto.NewCounterVec(counter("response", "errors"), []string{"code"}),
		RefreshCounter: promauto.NewCounterVec(counter("version", "refreshed"), []string{"product", "arch", "channel", "version"}),
		RefreshErrors:  promauto.NewCounterVec(counter("version", "refresh_errors"), []string{"product", "arch", "channel", "version"}),

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

func counter(subsystem, name string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func gauge(subsystem, name string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}

func histogram(subsystem, name string) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Namespace: METRIC_NAMESPACE,
		Subsystem: subsystem,
		Name:      name,
	}
}
