package controller

import (
	"context"
	"github.com/lukeelten/openshift-update-proxy/pkg/client"
	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"go.uber.org/zap"
	"time"
)

type CleanupController struct {
	Config *config.UpdateProxyConfig
	Logger *zap.SugaredLogger

	Cache *client.OpenShiftVersionCache
}

func NewCleanupController(cfg *config.UpdateProxyConfig, logger *zap.SugaredLogger, versionCache *client.OpenShiftVersionCache) *CleanupController {
	return &CleanupController{
		Config: cfg,
		Logger: logger,
		Cache:  versionCache,
	}
}

func (con *CleanupController) Run(ctx context.Context) error {
	con.Logger.Debug("start cleanup controller")

	for {

		select {
		case <-time.NewTimer(con.Config.Cache.ControllerCycle).C:
			continue

		case <-ctx.Done():
			return nil
		}

	}
}
