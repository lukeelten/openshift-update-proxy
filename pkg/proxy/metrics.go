package proxy

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

	Config *config.UpdateProxyConfig
	Server http.Server
}

func NewUpdateProxyMetrics(cfg *config.UpdateProxyConfig) *UpdateProxyMetrics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &UpdateProxyMetrics{
		updateInfo:   promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_cluster_infos"}, []string{"arch", "channel", "version"}),
		healthchecks: promauto.NewCounterVec(prometheus.CounterOpts{Name: "openshift_update_proxy_healthchecks"}, []string{"status"}),
		Server: http.Server{
			Handler: mux,
			Addr:    cfg.Metrics.Listen,
		},
	}
}

func (metrics *UpdateProxyMetrics) UpdateInfo(arch, channel, version string) {
	metrics.updateInfo.WithLabelValues(arch, channel, version).Inc()
}

func (metrics *UpdateProxyMetrics) Healthcheck(status bool) {
	metrics.healthchecks.WithLabelValues(strconv.FormatBool(status)).Inc()
}

func (metrics *UpdateProxyMetrics) Run() error {
	return metrics.Server.ListenAndServe()
}

func (metrics *UpdateProxyMetrics) Shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return metrics.Server.Shutdown(shutdownCtx)
}
