package controller

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/cache"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"go.uber.org/zap"
	"time"
)

type CleanupController struct {
	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Cache *cache.OpenShiftVersionCache
}

func NewCleanupController(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, versionCache *cache.OpenShiftVersionCache) *CleanupController {
	return &CleanupController{
		Config: cfg,
		Logger: logger,
		Cache:  versionCache,
	}
}

func (con *CleanupController) Run(ctx context.Context) error {
	con.Logger.Debug("start cleanup controller")

	for {
		now := time.Now()
		num := con.Cache.DeleteAll(func(entry *cache.VersionEntry) bool {
			evictionThreshold := entry.LastAccessed.Add(con.Config.Cache.EvictAfter)
			return evictionThreshold.After(now)
		})
		con.Logger.Debugw("Deleted entries", "num", num)
		if num > 0 {
			con.Logger.Infow("Deleted entries from cache", "entries", num)
		}

		select {
		case <-time.NewTimer(con.Config.Cache.ControllerCycle).C:
			continue

		case <-ctx.Done():
			return nil
		}

	}
}
