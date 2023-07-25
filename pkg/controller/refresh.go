package controller

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"github.com/lukeelten/openshift-update-proxy/pkg/metrics"
	"github.com/lukeelten/openshift-update-proxy/pkg/upstream"
	"go.uber.org/zap"
	"time"
)

type RefreshController struct {
	Config  *config.UpdateProxyConfig
	Logger  *zap.SugaredLogger
	Metrics *metrics.UpdateProxyMetrics
	Product string

	Cache  *cache.OpenShiftVersionCache
	Client *upstream.UpstreamClient
}

func NewRefreshController(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, metric *metrics.UpdateProxyMetrics, versionCache *cache.OpenShiftVersionCache, client *upstream.UpstreamClient) *RefreshController {
	return &RefreshController{
		Config:  cfg,
		Logger:  logger,
		Cache:   versionCache,
		Metrics: metric,
		Client:  client,
	}
}

func (con *RefreshController) Run(ctx context.Context) error {
	con.Logger.Debug("start refresh controller")

	for {
		now := time.Now()
		con.Cache.Foreach(func(entry *cache.VersionEntry) {
			if now.After(entry.ValidUntil) {
				con.Logger.Debugw("start refresh entry", "entry", *entry)

				body, err := con.Client.LoadVersionInfo(entry.Arch, entry.Channel, entry.Version)
				if err != nil {
					con.Metrics.RefreshErrors.WithLabelValues(con.Client.Product, entry.Arch, entry.Channel, entry.Version).Inc()
					con.Logger.Errorw("got error refreshing entry", "err", err, "arch", entry.Arch, "channel", entry.Channel, "version", entry.Version)
					return
				}

				entry.Body = body
				entry.ValidUntil = now.Add(con.Config.Cache.DefaultLifetime)
				con.Logger.Infow("successfully refreshed entry", "arch", entry.Arch, "channel", entry.Channel, "version", entry.Version)
				con.Metrics.RefreshCounter.WithLabelValues(con.Client.Product, entry.Arch, entry.Channel, entry.Version).Inc()
			}
		})

		select {
		case <-time.NewTimer(con.Config.Cache.ControllerCycle).C:
			continue

		case <-ctx.Done():
			return nil
		}

	}
}
